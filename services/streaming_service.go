package services

import (
    "bytes" // Added for ProcessList and improved Stderr handling
    "context"
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "sync" // Added for sync.Mutex
    "syscall"
    "time"

    log "github.com/sirupsen/logrus"
    "github.com/srad/mediasink/conf"
    "github.com/srad/mediasink/database"
    "github.com/srad/mediasink/helpers"
    "github.com/srad/mediasink/network"
)

type StreamInfo struct {
    IsOnline      bool                 `json:"isOnline" extensions:"!x-nullable"`
    IsTerminating bool                 `extensions:"!x-nullable"`
    URL           string               `extensions:"!x-nullable"`
    ChannelName   database.ChannelName `json:"channelName" extensions:"!x-nullable"`
}

type ProcessInfo struct {
    ID     database.ChannelID `json:"id"`
    Pid    int                `json:"pid"`
    Path   string             `json:"path"`
    Args   string             `json:"args"`
    Output string             `json:"output"`
}

var (
    // Package-level maps that need protection
    recInfo    = make(map[database.ChannelID]*database.Recording)
    streamInfo = make(map[database.ChannelID]StreamInfo)
    streams    = make(map[database.ChannelID]*exec.Cmd)

    // Mutexes for protecting concurrent access to the maps
    streamInfoLock sync.Mutex
    activeRecLock  sync.Mutex // Protects recInfo and streams
)

// Screenshot method on StreamInfo itself is fine as it operates on its own fields.
// The caller (startThumbnailWorker) will handle locking when accessing streamInfo map.
func (si *StreamInfo) Screenshot() error {
    // Ensure conf.FrameWidth and other path components are valid.
    // This function itself doesn't modify global maps.
    // Using absolute path for ffmpeg in helpers.ExtractFirstFrame is recommended.
    return helpers.ExtractFirstFrame(si.URL, conf.FrameWidth, filepath.Join(si.ChannelName.AbsoluteChannelDataPath(), database.SnapshotFilename))
}

// CaptureChannel Starts and also waits for the stream to end or being killed
func CaptureChannel(id database.ChannelID, url string, skip uint) error {
    channel, err := database.GetChannelByID(id)
    if err != nil {
        return fmt.Errorf("CaptureChannel: failed to get channel %d: %w", id, err)
    }

    activeRecLock.Lock() // Lock before checking and potentially modifying streams/recInfo
    if _, ok := streams[id]; ok {
        activeRecLock.Unlock() // Unlock if already capturing
        log.Debugf("CaptureChannel: Stream %d already in map, capture not starting again.", id)
        return nil // Not an error per se, just already being handled
    }
    // If not already streaming, keep lock for initial setup

    // Folder could not be created and does not exist yet.
    if errMkDir := channel.ChannelName.MkDir(); errMkDir != nil && !os.IsExist(errMkDir) {
        activeRecLock.Unlock() // Unlock before returning error
        return fmt.Errorf("CaptureChannel: failed to create directory for %s: %w", channel.ChannelName, errMkDir)
    }

    recording, outputFilePath, err := database.NewRecording(channel.ChannelID, "recording")
    if err != nil {
        activeRecLock.Unlock() // Unlock before returning error
        return fmt.Errorf("CaptureChannel: failed to create new recording entry for %s: %w", channel.ChannelName, err)
    }

    log.Infoln("----------------------------------------Capturing----------------------------------------")
    log.Infof("URL: %s", url)
    log.Infof("To: %s", outputFilePath)

    ffmpegExecutable := "/usr/local/bin/ffmpeg" // Use absolute path for robustness
    cmdArgs := []string{"-hide_banner", "-loglevel", "error", "-i", url, "-ss", fmt.Sprintf("%d", skip), "-movflags", "faststart", "-c", "copy", outputFilePath}
    cmdToRun := exec.Command(ffmpegExecutable, cmdArgs...)

    // Store in maps under lock
    recInfo[id] = recording
    streams[id] = cmdToRun
    activeRecLock.Unlock() // Unlock after map modifications, before blocking operations (Start/Wait)

    log.Infof("Executing: %s %s", ffmpegExecutable, strings.Join(cmdArgs, " "))

    var stderrBuf bytes.Buffer
    stderrPipe, pipeErr := cmdToRun.StderrPipe()
    if pipeErr != nil {
        log.Errorf("[Capture] Error creating stderr pipe for %s: %v. FFMPEG output may be lost.", channel.ChannelName, pipeErr)
        // Continue, but be aware stderr might not be captured.
    } else {
        // Asynchronously copy Stderr to the buffer.
        // This goroutine will exit once StderrPipe is closed (after cmdToRun.Wait() completes).
        go func() {
            _, errCopy := io.Copy(&stderrBuf, stderrPipe)
            if errCopy != nil {
                log.Warnf("[Capture] Error copying stderr for %s: %v", channel.ChannelName, errCopy)
            }
        }()
    }

    if err := cmdToRun.Start(); err != nil {
        // Log stderr if available, even if Start fails (though less likely to have output then)
        log.Errorf("[Capture] cmd.Start failed for %s: %v. Stderr: %s", channel.ChannelName, err, stderrBuf.String())
        // The calling goroutine in Start() will call DeleteStreamData to clean up map entries.
        return fmt.Errorf("ffmpeg cmd.Start failed for %s: %w", channel.ChannelName, err)
    }
    log.Infof("[Capture] ffmpeg process started for %s (PID: %d)", channel.ChannelName, cmdToRun.Process.Pid)

    waitErr := cmdToRun.Wait() // Wait for the command to finish

    // At this point, the stderr copying goroutine should have finished or will finish soon.
    // stderrBuf will contain the stderr output.
    stderrOutput := stderrBuf.String()
    if len(stderrOutput) > 0 {
        log.Warnf("[Capture] ffmpeg stderr for %s:\n%s", channel.ChannelName, stderrOutput)
    }

    if waitErr != nil {
        var exitErr *exec.ExitError
        // Check if it's an ExitError and if the code is 255 (often from os.Interrupt)
        if errors.As(waitErr, &exitErr) && exitErr.Sys().(syscall.WaitStatus).ExitStatus() == 255 {
            log.Infof("[Capture] ffmpeg for %s exited with status 255 (likely intentional stop via Interrupt).", channel.ChannelName)
        } else {
            log.Errorf("[Capture] ffmpeg process for '%s' exited with error: %v", channel.ChannelName, waitErr)
            // Attempt to remove the possibly corrupted/incomplete output file
            if errRemove := os.Remove(outputFilePath); errRemove != nil {
                log.Errorf("[Capture] Error deleting recording file '%s' after ffmpeg error: %v", outputFilePath, errRemove)
            }
            return fmt.Errorf("ffmpeg process for %s failed: %w", channel.ChannelName, waitErr)
        }
    } else {
        log.Infof("[Capture] ffmpeg process for %s finished successfully.", channel.ChannelName)
    }

    recDuration := time.Since(recording.CreatedAt)

    // Determine minimum required duration
    defaultMinDurationMinutes := 1.0 // 1min, default if DB query fails or not set
    channelDuration := defaultMinDurationMinutes

    currentChannelState, errChannel := database.GetChannelByID(id) // Re-fetch for latest MinDuration
    if errChannel == nil {
        channelDuration = float64(currentChannelState.MinDuration) // Use DB value
        log.Infof("[Capture] Minimum recording duration for channel %s is %f min (from DB).", currentChannelState.ChannelName, channelDuration)
    } else {
        log.Errorf("[Capture] Error querying channel %s (ID: %d) for MinDuration: %v. Using default %f min.", recording.ChannelName, id, errChannel, channelDuration)
    }

    if recDuration.Minutes() >= channelDuration {
        activeRecLock.Lock()
        recToFinalize, ok := recInfo[id]
        activeRecLock.Unlock()

        if ok && recToFinalize.Filename == recording.Filename {
            info := Info(id)
            if newRecording, err := database.CreateRecording(info.ChannelID, info.Filename, "recording"); err != nil {
                log.Errorf("[Info] Error adding recording '%s': %s", outputFilePath, err)
            } else {
                network.BroadCastClients(network.RecordingAddEvent, newRecording)

                if _, _, errPreviews := newRecording.EnqueuePreviewsJob(); errPreviews != nil {
                    return err
                }
            }
        } else { // Throw away
            log.Infof("[FinishRecording] Deleting stream '%s/%s' because it is too short (%dmin)", channel.ChannelName, recording.Filename, recDuration.Minutes())

            if err := os.Remove(outputFilePath); err != nil {
                log.Errorf("[Capture] Error destroying recording: %s", err)
            }
        }
    }
    return nil
}

func GetRecordingMinutes(id database.ChannelID) float64 {
    activeRecLock.Lock()
    defer activeRecLock.Unlock()
    if rec, ok := recInfo[id]; ok {
        // Also check if the stream process is still tracked
        if _, streamOk := streams[id]; streamOk {
            return time.Since(rec.CreatedAt).Minutes()
        }
    }
    return 0
}

func Info(id database.ChannelID) *database.Recording {
    activeRecLock.Lock()
    defer activeRecLock.Unlock()
    // The caller gets a pointer to the map's value.
    // If modification is possible and unintended, a copy should be returned.
    // For now, assuming read-only usage or intentional modification.
    return recInfo[id]
}

func Start(id database.ChannelID) (bool, error) {
    channel, err := database.GetChannelByID(id)
    if err != nil {
        return false, fmt.Errorf("start: failed to get channel %d: %w", id, err)
    }

    // Assuming id.PauseChannel(false) is a DB operation and thread-safe in itself
    if err := id.PauseChannel(false); err != nil {
        return false, fmt.Errorf("start: failed to unpause channel %d: %w", id, err)
    }

    url, queryErr := channel.QueryStreamURL() // Assuming QueryStreamURL is thread-safe

    // This was the panic site for "concurrent map writes"
    streamInfoLock.Lock()
    var currentIsTerminating bool
    // Check if entry exists to preserve IsTerminating if already set
    if siExisting, ok := streamInfo[channel.ChannelID]; ok {
        currentIsTerminating = siExisting.IsTerminating
    }
    streamInfo[channel.ChannelID] = StreamInfo{
        IsOnline:      url != "" && queryErr == nil, // Mark online only if URL found AND no query error
        URL:           url,
        ChannelName:   channel.ChannelName,
        IsTerminating: currentIsTerminating,
    }
    streamInfoLock.Unlock()

    if queryErr != nil {
        log.Warnf("[Start] URL query error for %s: %v. Stream marked as offline.", channel.ChannelName, queryErr)
        return false, queryErr // Return the queryErr so checkStreams can log it
    }
    if url == "" {
        log.Infof("[Start] No url found for channel: %s. Stream marked as offline.", channel.ChannelName)
        return false, nil // Not an error, just stream is offline
    }

    log.Infof("[Start] Initiating stream capture for '%s' at '%s'", channel.ChannelName, url)

    go func() {
        // This helpers.ExtractFirstFrame is for the live snapshot.
        // Ensure it uses absolute paths internally for ffmpeg.
        if errSnapshot := helpers.ExtractFirstFrame(url, conf.FrameWidth, filepath.Join(channel.ChannelName.AbsoluteChannelDataPath(), database.SnapshotFilename)); errSnapshot != nil {
            log.Errorf("[Start] Error extracting live snapshot for %s: %v", channel.ChannelName, errSnapshot)
        }
    }()

    go func() {
        log.Infof("[Start] Goroutine launched to capture channel %s (ID: %d), URL: %s", channel.ChannelName, id, url)
        if errCap := CaptureChannel(id, url, channel.SkipStart); errCap != nil {
            log.Errorf("[Start] CaptureChannel for %s (ID: %d) returned error: %v", channel.ChannelName, id, errCap)
        }
        // DeleteStreamData is crucial for cleanup after CaptureChannel completes or errors.
        // This ensures that IsRecordingStream will return false for this ID afterwards.
        DeleteStreamData(id)
        log.Infof("[Start] Goroutine for channel %s (ID: %d) finished, associated stream data deleted.", channel.ChannelName, id)
    }()

    return true, nil // Successfully initiated the start process
}

func TerminateAll() {
    activeRecLock.Lock()
    // Create a list of IDs to terminate to avoid holding lock while calling TerminateProcess,
    // as TerminateProcess itself acquires locks.
    idsToTerminate := make([]database.ChannelID, 0, len(streams))
    for id := range streams {
        idsToTerminate = append(idsToTerminate, id)
    }
    activeRecLock.Unlock()

    log.Infof("[TerminateAll] Attempting to terminate %d streams.", len(idsToTerminate))
    for _, id := range idsToTerminate {
        // TerminateProcess will handle its own logging for individual successes/failures
        if err := TerminateProcess(id); err != nil {
            // Log here if TerminateProcess itself returns an unexpected error (not just ffmpeg signal error)
            log.Errorf("[TerminateAll] Error returned by TerminateProcess for channel ID %d: %v", id, err)
        }
    }
    log.Infoln("[TerminateAll] Finished all termination attempts.")
}

func TerminateProcess(id database.ChannelID) error {
    activeRecLock.Lock()
    cmd, cmdExists := streams[id]
    // We need to know the channel name for logging, even if streamInfo is later not found.
    // It's safer to grab it here if cmdExists.
    var channelNameForLog database.ChannelName
    if rec, recExists := recInfo[id]; recExists {
        channelNameForLog = rec.ChannelName
    }
    activeRecLock.Unlock() // Release lock on streams/recInfo map

    if !cmdExists {
        log.Infof("[TerminateProcess] No active stream process found for channel ID %d to terminate.", id)
        return nil // Not an error if not currently recording
    }

    log.Infof("[TerminateProcess] Attempting to terminate process for channel ID %d (Name: %s).", id, channelNameForLog)

    streamInfoLock.Lock()
    if si, siExists := streamInfo[id]; siExists {
        if !si.IsTerminating { // Avoid redundant logging
            si.IsTerminating = true
            streamInfo[id] = si // Write back the modified struct
            log.Infof("[TerminateProcess] Marked channel %d (Name: %s) as IsTerminating in streamInfo.", id, si.ChannelName)
        }
    } else {
        // This case might happen if DeleteStreamData was called after a crash but before TerminateProcess.
        log.Warnf("[TerminateProcess] No streamInfo found for channel ID %d (Name: %s) when trying to mark IsTerminating.", id, channelNameForLog)
    }
    streamInfoLock.Unlock()

    if cmd.Process == nil {
        log.Warnf("[TerminateProcess] cmd.Process is nil for channel ID %d (Name: %s). Cannot send signal. Recording might have failed to start or already exited.", id, channelNameForLog)
        // If process is nil, it means it never started or already cleaned up.
        // The goroutine that called CaptureChannel should call DeleteStreamData.
        return fmt.Errorf("process for channel %d was nil, cannot interrupt", id)
    }

    log.Infof("[TerminateProcess] Sending Interrupt signal to process for channel ID %d (Name: %s, PID: %d).", id, channelNameForLog, cmd.Process.Pid)
    if err := cmd.Process.Signal(os.Interrupt); err != nil {
        // "os: process already finished" is a common and expected "error" if ffmpeg exited on its own.
        if strings.Contains(strings.ToLower(err.Error()), "process already finished") {
            log.Infof("[TerminateProcess] Process for channel ID %d (Name: %s) already finished when Interrupt was sent.", id, channelNameForLog)
        } else {
            // Other errors sending the signal are more problematic.
            log.Errorf("[TerminateProcess] Error sending Interrupt signal to process for channel ID %d (Name: %s): %v", id, channelNameForLog, err)
            return fmt.Errorf("failed to send interrupt to process for channel %d: %w", id, err)
        }
    } else {
        log.Infof("[TerminateProcess] Interrupt signal sent successfully to process for channel ID %d (Name: %s).", id, channelNameForLog)
    }
    // The goroutine running CaptureChannel is responsible for cmd.Wait() and calling DeleteStreamData.
    return nil
}

func IsOnline(id database.ChannelID) bool {
    streamInfoLock.Lock()
    defer streamInfoLock.Unlock()
    si, ok := streamInfo[id]
    return ok && si.IsOnline // Return true only if entry exists AND IsOnline is true
}

func IsTerminating(id database.ChannelID) bool {
    streamInfoLock.Lock()
    defer streamInfoLock.Unlock()
    si, ok := streamInfo[id]
    return ok && si.IsTerminating // Return true only if entry exists AND IsTerminating is true
}

func IsRecordingStream(id database.ChannelID) bool {
    activeRecLock.Lock()
    defer activeRecLock.Unlock()
    _, ok := streams[id]
    return ok
}

func DeleteStreamData(id database.ChannelID) {
    // This function is critical for cleanup and preventing stale state.
    log.Infof("[DeleteStreamData] Attempting to delete data for channel ID %d.", id)

    activeRecLock.Lock()
    // Log details if they exist before deleting
    if cmd, ok := streams[id]; ok && cmd.Process != nil {
        log.Debugf("[DeleteStreamData] Removing stream process entry for channel ID %d (PID: %d).", id, cmd.Process.Pid)
    } else if ok {
        log.Debugf("[DeleteStreamData] Removing stream command entry for channel ID %d (process was nil or not started).", id)
    }
    delete(streams, id)
    delete(recInfo, id)
    activeRecLock.Unlock()

    streamInfoLock.Lock()
    if si, ok := streamInfo[id]; ok {
        // Reset flags rather than deleting the whole entry, to keep ChannelName etc.
        // This helps if the channel is checked again soon.
        si.IsOnline = false
        si.IsTerminating = false
        si.URL = "" // Clear sensitive/stale data
        streamInfo[id] = si
        log.Infof("[DeleteStreamData] Marked streamInfo for channel %d (Name: %s) as offline, not terminating, and cleared URL.", id, si.ChannelName)
    } else {
        log.Warnf("[DeleteStreamData] No streamInfo found for channel %d to update/reset.", id)
    }
    streamInfoLock.Unlock()
    log.Infof("[DeleteStreamData] Finished deleting/resetting data for channel ID %d.", id)
}

func ProcessList() []*ProcessInfo {
    activeRecLock.Lock()
    // Create a snapshot to minimize lock duration
    type cmdDetail struct {
        id   database.ChannelID
        pid  int
        path string
        args []string
    }
    snapshot := make([]cmdDetail, 0, len(streams))
    for id, cmd := range streams {
        pidVal := 0 // Default if Process is nil
        if cmd.Process != nil {
            pidVal = cmd.Process.Pid
        }
        snapshot = append(snapshot, cmdDetail{id: id, pid: pidVal, path: cmd.Path, args: cmd.Args})
    }
    activeRecLock.Unlock()

    infoList := make([]*ProcessInfo, 0, len(snapshot))
    for _, detail := range snapshot {
        // CombinedOutput cannot be used on an already started/piped command.
        // Output field will be empty unless captured differently.
        infoList = append(infoList, &ProcessInfo{
            ID:     detail.id,
            Pid:    detail.pid,
            Path:   detail.path,                    // This is the command path, e.g., "/usr/local/bin/ffmpeg"
            Args:   strings.Join(detail.args, " "), // This includes the command itself as Args[0]
            Output: "",                             // Output needs to be captured during the command's run
        })
    }
    return infoList
}

// startThumbnailWorker Creates in intervals snapshots of the video as a preview.
func startThumbnailWorker(ctx context.Context) {
    log.Infoln("[startThumbnailWorker] Thumbnail worker started.")
    defer log.Infoln("[startThumbnailWorker] Thumbnail worker stopped.")

    ticker := time.NewTicker(captureThumbInterval) // Use the ticker
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            log.Infoln("[startThumbnailWorker] Context cancelled, stopping worker.")
            return
        case <-ticker.C: // Wait for ticker event
            streamInfoLock.Lock()
            // Create a copy of StreamInfo structs to process outside the lock.
            // This is important because info.Screenshot() might be a longer operation.
            infosToSnapshot := make(map[database.ChannelID]StreamInfo)
            for id, si := range streamInfo {
                // Only process online, non-terminating streams that have a URL.
                if si.IsOnline && si.URL != "" && !si.IsTerminating {
                    infosToSnapshot[id] = si // This creates a copy of the StreamInfo struct.
                }
            }
            streamInfoLock.Unlock()

            if len(infosToSnapshot) > 0 {
                log.Debugf("[startThumbnailWorker] Processing %d channels for live thumbnails.", len(infosToSnapshot))
            }
            for channelID, infoCopy := range infosToSnapshot { // Iterate over the copied structs
                log.Debugf("[startThumbnailWorker] Attempting live snapshot for channel %s (ID: %d)", infoCopy.ChannelName, channelID)
                // infoCopy is a copy, so Screenshot() operates on its fields safely.
                if err := infoCopy.Screenshot(); err != nil {
                    log.Errorf("[startThumbnailWorker] Error extracting live snapshot for channel %s (ID: %d): %v", infoCopy.ChannelName, channelID, err)
                } else {
                    log.Debugf("[startThumbnailWorker] Live snapshot successful for channel %s (ID: %d)", infoCopy.ChannelName, channelID)
                    network.BroadCastClients(network.ChannelThumbnailEvent, channelID) // Broadcast original channelID
                }
            }
        }
    }
}
