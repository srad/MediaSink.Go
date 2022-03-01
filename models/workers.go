package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/srad/streamsink/conf"
	"github.com/srad/streamsink/utils"
	"gorm.io/gorm"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	sleepBetweenRounds = 1 * time.Second
	threadCount        = uint(float32(runtime.NumCPU() / 2))
	cancelWorker       context.CancelFunc
)

type JsonFFProbeInfo struct {
	Streams []struct {
		Width      uint   `json:"width"`
		Height     uint   `json:"height"`
		RFrameRate string `json:"r_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

type FFProbeInfo struct {
	Fps      float64
	Duration float64
	Size     uint64
	BitRate  uint64
	Width    uint
	Height   uint
}

type CuttingJob struct {
	OnStart    func(*utils.CommandInfo)
	OnProgress func(string)
}

type CutArgs struct {
	Starts []string `json:"starts"`
	Ends   []string `json:"ends"`
}

type PreviewJob struct {
	OnStart     func(*utils.CommandInfo)
	OnProgress  func(*ProcessInfo)
	ChannelName string
	Filename    string
}

type ProcessInfo struct {
	JobType string
	Frame   int
	Total   int
	Raw     string
}

const (
	winFont   = "C\\\\:/Windows/Fonts/DMMono-Regular.ttf"
	linuxFont = "/usr/share/fonts/truetype/DMMono-Regular.ttf"
	// FrameCount Number of extracted frames or timeline/preview
	FrameCount = 96
)

func MergeVideos(outputListener func(string), absoluteMergeTextfile, absoluteOutputFilepath string) error {
	log.Println("---------------------------------------------- Merge Job ----------------------------------------------")
	log.Println(absoluteMergeTextfile)
	log.Println(absoluteOutputFilepath)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return utils.ExecSync(&utils.ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-hide_banner", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", absoluteMergeTextfile, "-movflags", "faststart", "-codec", "copy", absoluteOutputFilepath},
		OnStart: func(info utils.CommandInfo) {

		},
		OnPipeErr: func(info utils.PipeMessage) {
			outputListener(info.Message)
		},
	})
}

func CutVideo(args *CuttingJob, absoluteFilepath, absoluteOutputFilepath, startIntervals, endIntervals string) error {
	log.Println("---------------------------------------------- Cutting Job ----------------------------------------------")
	log.Println(absoluteFilepath)
	log.Println(absoluteOutputFilepath)
	log.Println(startIntervals)
	log.Println(endIntervals)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return utils.ExecSync(&utils.ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-progress", "pipe:2", "-hide_banner", "-loglevel", "error", "-i", absoluteFilepath, "-ss", startIntervals, "-to", endIntervals, "-movflags", "faststart", "-codec", "copy", absoluteOutputFilepath},
		OnStart: func(info utils.CommandInfo) {
			args.OnStart(&info)
		},
		OnPipeErr: func(info utils.PipeMessage) {
			args.OnProgress(info.Message)
		},
	})
}

func getFontPath() string {
	if runtime.GOOS == "windows" {
		return winFont
	}
	return linuxFont
}

func CreatePreviewStripe(errListener func(string), outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) error {
	dir := filepath.Join(outputDir, conf.StripesFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return utils.ExecSync(&utils.ExecArgs{
		OnPipeErr: func(info utils.PipeMessage) {
			errListener(info.Message)
		},
		Command:     "ffmpeg",
		CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:2", "-frames:v", "1", "-q:v", "0", "-threads", fmt.Sprint(threadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,tile=%dx1", frameDistance, frameHeight, getFontPath(), fps, FrameCount), "-hide_banner", "-loglevel", "error", "-stats", "-vsync", "vfr", filepath.Join(dir, outFile)},
	})
}

func CreatePreviewPoster(inputPath, outputDir, filename string) error {
	dir := filepath.Join(outputDir, conf.PostersFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return ExtractFirstFrame(inputPath, frameWidth, filepath.Join(dir, filename))
}

func CreatePreviewVideo(pipeInfo func(info utils.PipeMessage), outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) error {
	dir := filepath.Join(outputDir, conf.VideosFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return utils.ExecSync(&utils.ExecArgs{
		OnPipeErr:   pipeInfo,
		Command:     "ffmpeg",
		CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:2", "-q:v", "0", "-threads", fmt.Sprint(threadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,setpts=(7/2)*N/TB", frameDistance, frameHeight, getFontPath(), fps), "-hide_banner", "-loglevel", "error", "-stats", "-vsync", "vfr", "-movflags", "faststart", filepath.Join(dir, outFile)},
	})
}

func calcFps(output string) (float64, error) {
	numbers := strings.Split(output, "/")

	if len(numbers) != 2 {
		return 0, errors.New("ffprobe output is not as expected a divison: a/b")
	}

	a, err := strconv.ParseFloat(numbers[0], 32)
	if err != nil {
		return 0, err
	}

	b, err := strconv.ParseFloat(numbers[1], 32)
	if err != nil {
		return 0, err
	}

	fps := a / b

	return fps, nil
}

func ExtractFirstFrame(input, height, output string) error {
	err := utils.ExecSync(&utils.ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-y", "-hide_banner", "-loglevel", "error", "-i", input, "-r", "1", "-vf", "scale=" + height + ":-1", "-q:v", "2", "-frames:v", "1", output},
	})

	if err != nil {
		log.Printf("[Recorder] Error extracting frame: %v", err.Error())
		return nil
	}

	return nil
}

func StartWorker() {
	ctx, c := context.WithCancel(context.Background())
	cancelWorker = c
	go processJobs(ctx)
}

func StopWorker() {
	cancelWorker()
}

func processJobs(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("[processJobs] Worker stopped")
			return
		case <-time.After(sleepBetweenRounds):
			cuttingJobs()
			previewJobs()
		}
	}
}

// Handles one single job.
func previewJobs() {
	job, err := GetNextJob(StatusPreview)
	if job == nil && err == nil {
		// log.Printf("No jobs found with status '%s'", models.StatusPreview)
		return
	}
	if err != nil {
		log.Printf("[Job] Error handlung job: %v", err)
		return
	}

	// Delete any old previews first
	errDelete := DestroyPreviews(job.ChannelName, job.Filename)
	if errDelete != nil && errDelete != gorm.ErrRecordNotFound {
		log.Printf("[Job] Error deleting existing previews: %v", errDelete)
	}

	log.Printf("[Job] Generating preview for '%s'", job.Filename)
	err = ActiveJob(job.JobId)
	if err != nil {
		log.Printf("[Job] Error activating job: %d", job.JobId)
	}

	err = GeneratePreviews(&PreviewJob{
		OnStart: func(info *utils.CommandInfo) {
			_ = job.UpdateInfo(info.Pid, info.Command)
			notify("job:preview:start", JobMessage{jobId: job.JobId, ChannelName: job.ChannelName, Filename: job.Filename})
		},
		OnProgress: func(info *ProcessInfo) {
			_ = job.UpdateProgress(fmt.Sprintf("%f", float32(info.Frame)/float32(info.Total)*100))
			notify("job:preview:progress", JobMessage{jobId: job.JobId, ChannelName: job.ChannelName, Filename: job.Filename})
		},
		ChannelName: job.ChannelName,
		Filename:    job.Filename,
	})
	if err != nil {
		// Delete the file if it is corrupted
		checkFileErr := CheckVideo(conf.GetRecordingsPaths(job.ChannelName, job.Filename).Filepath)
		if checkFileErr != nil {
			if rec, err := job.FindRecording(); err != nil {
				_ = rec.Destroy()
			}
			log.Printf("[Job] File corrupted, deleting '%s', %v\n", job.Filename, checkFileErr)
		}
		// Since the job failed for some reason, remove it
		_ = job.Destroy()
		log.Printf("[Job] Error generating preview for '%s' : %v\n", job.Filename, err)
		return
	}

	_, err2 := UpdatePreview(job.ChannelName, job.Filename)
	if err2 != nil {
		log.Printf("[Job] Error adding previews: %v", err2)
		return
	}

	if rec, err := job.FindRecording(); err != nil {
		notify("job:preview:done", JobMessage{jobId: job.JobId, ChannelName: job.ChannelName, Filename: job.Filename, Data: rec})
	}
	err3 := job.Destroy()
	if err3 != nil {
		log.Printf("[Job] Error deleteing job: %v", err3)
		return
	}

	log.Printf("[Job] Preview job complete for '%s'", job.Filepath)
}

// Cut video, add preview job, destroy job.
// This action is intrinsically procedural, keep it together locally.
func cuttingJobs() error {
	job, err := GetNextJob(StatusCut)
	if err == gorm.ErrRecordNotFound || job == nil {
		return err
	}

	if err != nil {
		log.Printf("[Job] Error handling cutting job: %v", err)
		return err
	}

	log.Printf("[Job] Generating preview for '%s'", job.Filename)
	err = ActiveJob(job.JobId)
	if err != nil {
		log.Printf("[Job] Error activating job: %d", job.JobId)
	}

	if job.Args == nil {
		log.Printf("[Job] Error missing args for cutting job: %d", job.JobId)
		return err
	}

	// Parse arguments
	cutArgs := &CutArgs{}
	s := []byte(*job.Args)
	err = json.Unmarshal(s, &cutArgs)
	if err != nil {
		log.Printf("[Job] Error parsing cutting job arguments: %v", err)
		_ = job.Destroy()
		return err
	}

	// Filenames
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	filename := fmt.Sprintf("%s_cut_%s.mp4", job.ChannelName, stamp)
	inputPath := conf.AbsoluteFilepath(job.ChannelName, job.Filename)
	outputFile := conf.AbsoluteFilepath(job.ChannelName, filename)
	segFiles := make([]string, len(cutArgs.Starts))
	mergeFileContent := make([]string, len(cutArgs.Starts))

	// Cut
	segmentFilename := fmt.Sprintf("%s_cut_%s", job.ChannelName, stamp)
	for i, start := range cutArgs.Starts {
		segFiles[i] = conf.AbsoluteFilepath(job.ChannelName, fmt.Sprintf("%s_%04d.mp4", segmentFilename, i))
		err = CutVideo(&CuttingJob{
			OnStart: func(info *utils.CommandInfo) {
				_ = job.UpdateInfo(info.Pid, info.Command)
			},
			OnProgress: func(s string) {
				log.Printf("[CutVideo] %s", s)
			},
		}, inputPath, segFiles[i], start, cutArgs.Ends[i])
		// Failed, delete all segments
		if err != nil {
			log.Printf("[Job] Error generating cut for file '%s': %v", inputPath, err)
			log.Println("[Job] Deleting orphaned segments")
			for _, file := range segFiles {
				if err := os.RemoveAll(file); err != nil {
					log.Printf("[Job] Error deleting segment '%s': %v", file, err)
				}
			}
			_ = job.Destroy()
			return err
		}
	}
	// Merge file txt, enumerate
	for i, file := range segFiles {
		mergeFileContent[i] = fmt.Sprintf("file '%s'", file)
	}
	mergeTextfile := conf.AbsoluteFilepath(job.ChannelName, fmt.Sprintf("%s.txt", segmentFilename))
	err = os.WriteFile(mergeTextfile, []byte(strings.Join(mergeFileContent, "\n")), 0644)
	if err != nil {
		log.Printf("[Job] Error writing concat text file '%s': %v", mergeTextfile, err)
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Printf("[Job] Error deleting %s: %v", file, err)
			}
		}
		_ = job.Destroy()
		return err
	}

	err = MergeVideos(func(s string) {
		log.Printf("[MergeVideos] %s", s)
	}, mergeTextfile, outputFile)
	if err != nil {
		log.Printf("[Job] Error merging file '%s': %s", mergeTextfile, err.Error())
		for _, file := range segFiles {
			if err := os.RemoveAll(file); err != nil {
				log.Printf("[Job] Error deleting %s: %s", file, err.Error())
			}
		}
		_ = job.Destroy()
		return err
	}
	_ = os.RemoveAll(mergeTextfile)
	for _, file := range segFiles {
		if err := os.Remove(file); err != nil {
			log.Printf("[Job] Error deleting segment '%s': %s", file, err.Error())
		} else {
			log.Printf("[Job] Deleted segment '%s': %s", file, err.Error())
		}
	}

	info, err := GetVideoInfo(outputFile)
	if err != nil {
		log.Printf("[Job] Error reading video information for file '%s': %v", filename, err)
	}

	// Cutting written to dist, add record to database
	newRec := Recording{
		ChannelName:  job.ChannelName,
		Filename:     filename,
		PathRelative: conf.GetRelativeRecordingsPath(job.ChannelName, filename),
		Duration:     info.Duration,
		Width:        info.Width,
		Height:       info.Height,
		Size:         info.Size,
		BitRate:      info.BitRate,
		CreatedAt:    time.Now(),
		Bookmark:     false,
	}

	err = newRec.Save("cut")
	if err != nil {
		log.Printf("[Job] Error creating: %v", err)
		return err
	}

	// Successfully added cut record, enqueue preview job
	_, err = EnqueuePreviewJob(job.ChannelName, filename)
	if err != nil {
		log.Printf("[Job] Error adding preview for cutting job %d: %v", job.JobId, err)
		return err
	}

	// Finished, destroy job
	err = job.Destroy()
	if err != nil {
		log.Printf("[Job] Error deleteing job: %v", err)
		return err
	}

	log.Printf("[Job] Cutting job complete for '%s'", job.Filepath)
	return nil
}

func CheckVideo(filepath string) error {
	return utils.ExecSync(&utils.ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-v", "error", "-i", filepath, "-f", "null", "-"},
	})
}

func GeneratePreviews(args *PreviewJob) error {
	inputPath := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, args.ChannelName, args.Filename)

	log.Println("---------------------------------------------- Preview Job ----------------------------------------------")
	log.Println(inputPath)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return ExtractFrames(args, inputPath, conf.AbsoluteDataPath(args.ChannelName), FrameCount, 128, 256)
}

func ExtractFrames(args *PreviewJob, inputPath, outputDir string, extractCount int, frameHeight, videoHeight uint) error {
	totalFrameCount, err := GetFrameCount(inputPath)
	if err != nil {
		log.Printf("Error getting frame count for: '%s'\n", inputPath)
		return err
	}

	info, err := GetVideoInfo(inputPath)
	if err != nil {
		log.Printf("Error getting frame rate for: '%s'\n", inputPath)
		return err
	}

	frameDistance := uint(float32(totalFrameCount) / float32(extractCount))
	basename := filepath.Base(inputPath)
	filename := utils.FileNameWithoutExtension(basename)

	if err := CreatePreviewStripe(func(s string) {
		log.Printf("[createPreviewStripe] %s", s)
	}, outputDir, filename+".jpg", inputPath, frameDistance, frameHeight, info.Fps); err != nil {
		return errors.New(fmt.Sprintf("error generating stripe for '%s': %s", inputPath, err.Error()))
	}

	i := 1
	if err := CreatePreviewVideo(func(info utils.PipeMessage) {
		if strings.Contains(info.Message, "frame=") {
			args.OnProgress(&ProcessInfo{Frame: i, Raw: info.Message, Total: extractCount})
			i++
		}
	}, outputDir, filename+".mp4", inputPath, frameDistance, videoHeight, info.Fps); err != nil {
		return errors.New(fmt.Sprintf("error generating preview video for '%s': %s", inputPath, err.Error()))
	}

	if err := CreatePreviewPoster(inputPath, outputDir, filename+".jpg"); err != nil {
		return errors.New(fmt.Sprintf("error generating poster for '%s': %s", inputPath, err.Error()))
	}

	return nil
}

// GetFrameCount This requires an entire video passthrough
func GetFrameCount(filepath string) (uint64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=nb_read_packets", "-of", "csv=p=0", "-select_streams", "v:0", "-count_packets", filepath)
	stdout, err := cmd.Output()
	output := strings.TrimSpace(string(stdout))

	if err != nil {
		return 0, err
	}

	fps, err := strconv.ParseUint(output, 10, 64)
	if err != nil {
		return 0, nil
	}

	return fps, nil
}

func GetVideoInfo(filepath string) (*FFProbeInfo, error) {
	cmd := exec.Command("ffprobe", "-i", filepath, "-show_entries", "format=bit_rate,size,duration", "-show_entries", "stream=r_frame_rate,width,height", "-v", "error", "-select_streams", "v:0", "-of", "default=noprint_wrappers=1", "-print_format", "json")
	stdout, err := cmd.Output()
	output := strings.TrimSpace(string(stdout))

	if err != nil {
		return nil, err
	}

	parsed := &JsonFFProbeInfo{}
	err = json.Unmarshal([]byte(output), &parsed)
	if err != nil {
		return nil, err
	}

	info := &FFProbeInfo{
		BitRate:  0,
		Size:     0,
		Height:   0,
		Width:    0,
		Duration: 0,
		Fps:      0,
	}

	duration, err := strconv.ParseFloat(parsed.Format.Duration, 64)
	if err != nil {
		return info, err
	}
	info.Duration = duration

	bitrate, err := strconv.ParseUint(parsed.Format.BitRate, 10, 64)
	if err != nil {
		return info, err
	}
	info.BitRate = bitrate

	size, err := strconv.ParseUint(parsed.Format.Size, 10, 64)
	if err != nil {
		return info, err
	}
	info.Size = size

	fps, err := calcFps(parsed.Streams[0].RFrameRate)
	if err != nil {
		return info, err
	}
	info.Fps = fps

	info.Width = parsed.Streams[0].Width
	info.Height = parsed.Streams[0].Height

	return info, nil
}
