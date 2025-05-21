package services

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/srad/mediasink/database" // Assuming database.Channel has ChannelID, ChannelName, IsPaused
	"github.com/srad/mediasink/helpers"
	"github.com/srad/mediasink/network"
)

const (
	streamCheckBreak         = 2 * time.Second  // Delay after attempting to start a stream
	breakBetweenCheckStreams = 10 * time.Second // Interval for the main stream checking loop
	captureThumbInterval     = 30 * time.Second // Interval for thumbnail worker (implementation not shown)
	maxConcurrentChecks      = 5                // Max number of concurrent stream checks/start attempts
)

// recorderControlMessage defines the type for control messages sent to the stream worker.
type recorderControlMessage int

const (
	resumeRecording recorderControlMessage = iota // Signal to resume stream checking and recording
	pauseRecording                                // Signal to pause stream checking and recording
)

var (
	workerCancel   context.CancelFunc          // Function to cancel the worker goroutines
	controlChannel chan recorderControlMessage // Channel to send control messages to startStreamWorker
	recorderActive bool                        // Indicates if the recorder system (workers) is active
	recorderLock   sync.Mutex                  // Protects recorderActive, workerCancel, and controlChannel initialization
)

// StartRecorder initializes and starts the recording workers.
func StartRecorder() {
	recorderLock.Lock()
	defer recorderLock.Unlock()

	if recorderActive {
		log.Infoln("[Recorder] Recorder is already active.")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	workerCancel = cancel
	// Buffered channel to prevent deadlocks if sending a command when worker is busy
	controlChannel = make(chan recorderControlMessage, 1)

	go startStreamWorker(ctx, controlChannel)
	// Assuming startThumbnailWorker is also context-aware and part of the recorder system
	go startThumbnailWorker(ctx) // This was in the original StartRecorder

	recorderActive = true
	log.Infoln("[Recorder] Started recording workers.")
}

// StopRecorder signals the recording workers to stop and cleans up.
func StopRecorder() {
	recorderLock.Lock()
	defer recorderLock.Unlock()

	if !recorderActive {
		log.Infoln("[Recorder] Recorder not active or already stopped.")
		return
	}

	log.Infoln("[Recorder] Stopping recorder workers...")

	if workerCancel != nil {
		workerCancel() // Signal cancellation to all worker goroutines
	}

	// TerminateAll() is responsible for stopping any ongoing external processes (e.g., FFMPEG).
	TerminateAll()

	recorderActive = false
	// For a fully graceful shutdown, consider using a sync.WaitGroup for worker goroutines.
	log.Infoln("[Recorder] Recorder workers stopping sequence initiated.")
}

func startStreamWorker(ctx context.Context, ctrlChan <-chan recorderControlMessage) {
	log.Infoln("[startStreamWorker] Worker started.")
	var workerPaused = false // Internal pause state for this worker

	ticker := time.NewTicker(breakBetweenCheckStreams)
	defer ticker.Stop()
	defer log.Infoln("[startStreamWorker] Worker stopped.")

	for {
		select {
		case <-ctx.Done():
			log.Infoln("[startStreamWorker] Context cancelled, stopping worker.")
			return

		case cmd := <-ctrlChan:
			switch cmd {
			case pauseRecording:
				log.Infoln("[startStreamWorker] Received pause command. Pausing stream checks.")
				workerPaused = true
			case resumeRecording:
				log.Infoln("[startStreamWorker] Received resume command. Resuming stream checks.")
				workerPaused = false
			}

		case <-ticker.C:
			if workerPaused {
				log.Infoln("[startStreamWorker] Paused, skipping stream check cycle.")
				continue
			}
			log.Infoln("[startStreamWorker] Starting stream check cycle...")
			checkStreams(ctx) // Pass context for cancellation within checkStreams concurrent operations
			log.Infoln("[startStreamWorker] Stream check cycle finished. Waiting for next tick or command.")
		}
	}
}

// checkStreams iterates all enabled channels, fetches their latest state,
// checks their status concurrently, and starts recording if appropriate.
func checkStreams(ctx context.Context) {
	initialChannelList, err := database.EnabledChannelList()
	if err != nil {
		log.Errorf("[checkStreams] Error fetching enabled channel list: %v", err)
		return
	}

	if len(initialChannelList) == 0 {
		log.Debugln("[checkStreams] No enabled channels to check.")
		return
	}

	// Semaphore to limit the number of concurrent goroutines.
	sem := make(chan struct{}, maxConcurrentChecks)
	var wg sync.WaitGroup

	for _, initialChannelInfo := range initialChannelList {
		// Check for context cancellation before spawning each goroutine.
		select {
		case <-ctx.Done():
			log.Infoln("[checkStreams] Context cancelled during channel iteration, stopping further checks.")
			return
		default:
		}

		wg.Add(1)
		go func(channelFromList *database.Channel) { // Pass initial channel info by value
			defer wg.Done()
			sem <- struct{}{}        // Acquire semaphore slot
			defer func() { <-sem }() // Release semaphore slot

			// Check context again inside the goroutine, as it might take time to acquire semaphore.
			select {
			case <-ctx.Done():
				log.Infof("[checkStreams] Context cancelled before processing channel %s.", channelFromList.ChannelName)
				return
			default:
			}

			// Re-fetch the channel's current state to ensure up-to-date info, as per original logic.
			currentChannelState, fetchErr := database.GetChannelByID(channelFromList.ChannelID)
			if fetchErr != nil {
				log.Errorf("[checkStreams] Error fetching current state for channel %s (ID: %d): %v", channelFromList.ChannelName, channelFromList.ChannelID, fetchErr)
				return // Skip this channel
			}

			if IsRecordingStream(currentChannelState.ChannelID) {
				log.Debugf("[checkStreams] Channel %s (ID: %d) is already recording. Skipping.", currentChannelState.ChannelName, currentChannelState.ChannelID)
				return
			}
			if currentChannelState.IsPaused {
				log.Debugf("[checkStreams] Channel %s (ID: %d) is marked as paused in database. Skipping.", currentChannelState.ChannelName, currentChannelState.ChannelID)
				return
			}

			log.Infof("[checkStreams] Attempting to start stream for channel: %s (ID: %d)", currentChannelState.ChannelName, currentChannelState.ChannelID)

			// Preserving the original logic for handling Start() return values and broadcasting.
			started, startErr := Start(currentChannelState.ChannelID)

			if started && startErr != nil {
				log.Warnf("[checkStreams] Attempted to start channel %s (ID: %d), but received an error. Broadcasting offline. Error: %v", currentChannelState.ChannelName, currentChannelState.ChannelID, startErr)
				network.BroadCastClients(network.ChannelOfflineEvent, currentChannelState.ChannelID)
			} else if started { // Implies started == true && startErr == nil
				log.Infof("[checkStreams] Successfully started stream for channel %s (ID: %d). Broadcasting online and start events.", currentChannelState.ChannelName, currentChannelState.ChannelID)
				network.BroadCastClients(network.ChannelOnlineEvent, currentChannelState.ChannelID)
				network.BroadCastClients(network.ChannelStartEvent, currentChannelState.ChannelID)
			}
			if startErr != nil {
				log.Warnf("Did not start stream: %s", startErr.Error())
			}
			// If !started, the original code did not broadcast anything from this block.

			// Sleep after each attempt, as in the original sequential loop.
			// This might be for rate-limiting the Start() calls.
			time.Sleep(streamCheckBreak)

		}(initialChannelInfo)
	}

	wg.Wait() // Wait for all spawned goroutines for this check cycle to complete.
	log.Debugln("[checkStreams] Finished all concurrent checks for this cycle.")
}

// GeneratePosters generates preview posters for all existing recordings.
func GeneratePosters() error {
	log.Infoln("[GeneratePosters] Starting to update poster images for all recordings.")
	recordings, err := database.RecordingsList()
	if err != nil {
		log.Errorf("[GeneratePosters] Error fetching recordings list: %v", err)
		return err
	}

	if len(recordings) == 0 {
		log.Infoln("[GeneratePosters] No recordings found to generate posters for.")
		return nil
	}

	count := len(recordings)
	log.Infof("[GeneratePosters] Found %d recordings to process.", count)

	for i, rec := range recordings {
		filepath := rec.AbsoluteChannelFilepath() // Assuming rec has this method
		log.Infof("[GeneratePosters] Processing (%d/%d): %s", i+1, count, filepath)

		video := &helpers.Video{FilePath: filepath}

		if _, err := video.ExecPreviewCover(rec.DataFolder()); err != nil { // Assuming rec.DataFolder()
			log.Errorf("[GeneratePosters] Error creating poster for %s: %v", filepath, err)
		}
	}

	log.Infoln("[GeneratePosters] Finished generating posters for all applicable recordings.")
	return nil
}

func IsRecorderActive() bool {
	recorderLock.Lock()
	defer recorderLock.Unlock()
	return recorderActive
}
