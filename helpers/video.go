package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego/utils"
	"github.com/srad/streamsink/conf"
)

type CuttingJob struct {
	OnStart    func(*CommandInfo)
	OnProgress func(string)
}

type CutArgs struct {
	Starts []string `json:"starts"`
	Ends   []string `json:"ends"`
}

type VideoConversionArgs struct {
	OnStart     func(*CommandInfo)
	OnProgress  func(*ProcessInfo)
	OnEnd       func()
	ChannelName string
	Filename    string
}

type ProcessInfo struct {
	JobType string
	Frame   uint64
	Total   int
	Raw     string
}

type JsonFFProbeInfo struct {
	Streams []struct {
		Width       uint   `json:"width"`
		Height      uint   `json:"height"`
		RFrameRate  string `json:"r_frame_rate"`
		PacketCount string `json:"nb_read_packets"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

type FFProbeInfo struct {
	Fps         float64
	Duration    float64
	Size        uint64
	BitRate     uint64
	Width       uint
	Height      uint
	PacketCount uint64
}

type ConversionResult struct {
	ChannelName  string
	Filename     string
	PathRelative string
	Filepath     string
	CreatedAt    time.Time
}

type PreviewResult struct {
	ScreensPath    string
	StripeFilePath string
	VideoFilePath  string
	ChannelName    string
	Filename       string
}

func CreatePreviewStripe(errListener func(string), outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) error {
	dir := filepath.Join(outputDir, conf.StripesFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return ExecSync(&ExecArgs{
		OnPipeErr: func(info PipeMessage) {
			errListener(info.Output)
		},
		Command:     "ffmpeg",
		CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:2", "-frames:v", "1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,tile=%dx1", frameDistance, frameHeight, conf.FrameCount), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", filepath.Join(dir, outFile)},
		//CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:2", "-frames:v", "1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,tile=%dx1", frameDistance, frameHeight, conf.GetFontPath(), fps, conf.FrameCount), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", filepath.Join(dir, outFile)},
	})
}

func CreatePreviewPoster(inputPath, outputDir, filename string) error {
	dirPoster := filepath.Join(outputDir, conf.PostersFolder)
	if err := os.MkdirAll(dirPoster, 0777); err != nil {
		return err
	}

	return ExtractFirstFrame(inputPath, conf.FrameWidth, filepath.Join(dirPoster, filename))
}

func CreatePreviewVideo(pipeInfo func(info PipeMessage), outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) (string, error) {
	dir := filepath.Join(outputDir, conf.VideosFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return "", err
	}

	return dir, ExecSync(&ExecArgs{
		OnPipeErr:   pipeInfo,
		Command:     "ffmpeg",
		CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:2", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d", frameDistance, frameHeight), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", "-movflags", "faststart", filepath.Join(dir, outFile)},
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

func ExtractFirstFrame(input, height, outputPathPoster string) error {
	err := ExecSync(&ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-y", "-hide_banner", "-loglevel", "error", "-i", input, "-r", "1", "-vf", "scale=" + height + ":-1", "-q:v", "2", "-frames:v", "1", outputPathPoster},
	})

	if err != nil {
		return errors.New(fmt.Sprintf("[ExtractFirstFrame] Error extracting frame: %v", err.Error()))
	}

	return nil
}

func ConvertVideo(args *VideoConversionArgs, mediaType string) (*ConversionResult, error) {
	absoluteChannelFolder := conf.AbsoluteChannelPath(args.ChannelName)
	input := filepath.Join(absoluteChannelFolder, args.Filename)
	if !utils.FileExists(input) {
		return nil, errors.New(fmt.Sprintf("File '%s' does not exit", input))
	}

	// Might seem redundant, but since we have no dependent types...
	if mediaType == "mp3" {
		mp3Filename := fmt.Sprintf("%s.mp3", FileNameWithoutExtension(args.Filename))
		outputAbsoluteMp3 := filepath.Join(absoluteChannelFolder, mp3Filename)

		result := &ConversionResult{
			ChannelName:  args.ChannelName,
			Filename:     mp3Filename,
			PathRelative: conf.ChannelPath(args.ChannelName, mp3Filename),
			CreatedAt:    time.Now(),
			Filepath:     outputAbsoluteMp3,
		}

		err := ExecSync(&ExecArgs{
			OnPipeErr: func(info PipeMessage) {
			},
			OnStart: func(info CommandInfo) {
				args.OnStart(&info)
			},
			Command:     "ffmpeg",
			CommandArgs: []string{"-i", input, "-y", "-threads", fmt.Sprint(conf.ThreadCount), "-hide_banner", "-loglevel", "error", "-progress", "pipe:2", "-q:a", "0", "-map", "a", outputAbsoluteMp3},
		})

		return result, err
	} else {
		// video, anything else is a resolution
		// Create new filename
		name := fmt.Sprintf("%s_%s.mp4", FileNameWithoutExtension(args.Filename), mediaType)
		output := filepath.Join(absoluteChannelFolder, name)

		result := &ConversionResult{
			ChannelName:  args.ChannelName,
			Filename:     name,
			PathRelative: conf.ChannelPath(args.ChannelName, name),
			CreatedAt:    time.Now(),
			Filepath:     output,
		}

		err := ExecSync(&ExecArgs{
			OnPipeErr: func(info PipeMessage) {
				if strings.Contains(info.Output, "=") {
					kv := strings.Split(info.Output, "=")
					if len(kv) > 1 && kv[0] == "frame" {
						frame, err := strconv.ParseUint(kv[1], 10, 64)
						if err == nil {
							args.OnProgress(&ProcessInfo{Frame: frame})
						}
					}
				}
			},
			OnStart: func(info CommandInfo) {
				args.OnStart(&info)
			},
			Command: "ffmpeg",
			// Preset values: https://trac.ffmpeg.org/wiki/Encode/H.264
			// ultrafast
			// superfast
			// veryfast
			// faster
			// fast
			// medium – default preset
			// slow
			// slower
			// veryslow
			CommandArgs: []string{"-i", input, "-y", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("scale=-1:%s", mediaType), "-hide_banner", "-loglevel", "error", "-progress", "pipe:2", "-movflags", "faststart", "-c:v", "libx264", "-crf", "18", "-preset", "medium", "-c:a", "copy", output},
		})

		return result, err
	}
}

func CreatePreview(args *VideoConversionArgs, inputFilePath, outputDir string, extractCount int, frameHeight, videoHeight uint) (*PreviewResult, error) {
	totalFrameCount, err := GetFrameCount(inputFilePath)
	if err != nil {
		log.Printf("Error getting frame count for: '%s'\n", inputFilePath)
		return nil, err
	}

	info, err := GetVideoInfo(inputFilePath)
	if err != nil {
		log.Printf("Error getting frame rate for: '%s'\n", inputFilePath)
		return nil, err
	}

	frameDistance := uint(float32(totalFrameCount) / float32(extractCount))
	basename := filepath.Base(inputFilePath)
	filename := FileNameWithoutExtension(basename)

	if err := CreatePreviewStripe(func(s string) {
		if strings.Contains(s, "frame") {
			log.Printf("[createPreviewStripe] %s", s)
		}
	}, outputDir, filename+".jpg", inputFilePath, frameDistance, frameHeight, info.Fps); err != nil {
		return nil, errors.New(fmt.Sprintf("error generating stripe for '%s': %s", inputFilePath, err.Error()))
	}

	dir, err := CreatePreviewShots(func(s string) {}, outputDir, filename, inputFilePath, frameDistance, frameHeight, info.Fps)
	if err != nil {
		return nil, err
	}

	var i uint64 = 1
	previewVideoDir, err := CreatePreviewVideo(func(info PipeMessage) {
		if strings.Contains(info.Output, "frame=") {
			args.OnProgress(&ProcessInfo{Frame: i, Raw: info.Output, Total: extractCount})
			i++
		}
	}, outputDir, filename+".mp4", inputFilePath, frameDistance, videoHeight, info.Fps)

	if err != nil {
		return nil, errors.New(fmt.Sprintf("error generating preview video for '%s': %s", inputFilePath, err.Error()))
	}

	if err := CreatePreviewPoster(inputFilePath, outputDir, filename+".jpg"); err != nil {
		return nil, errors.New(fmt.Sprintf("error generating poster for '%s': %s", inputFilePath, err.Error()))
	}

	return &PreviewResult{
		ChannelName:    args.ChannelName,
		Filename:       args.Filename,
		ScreensPath:    dir,
		VideoFilePath:  previewVideoDir,
		StripeFilePath: path.Join(outputDir, filename+".jpg"),
	}, nil
}

// CreatePreviewShots Create a separate preview image file, at every frame distance.
func CreatePreviewShots(errListener func(s string), outputDir string, filename string, inputFilePath string, frameDistance uint, frameHeight uint, fps float64) (string, error) {
	dirPreview := filepath.Join(outputDir, conf.ScreensFolder, filename)
	if err := os.MkdirAll(dirPreview, 0777); err != nil {
		return dirPreview, err
	}

	outFile := fmt.Sprintf("%s_%%010d.jpg", filename)

	return dirPreview, ExecSync(&ExecArgs{
		OnPipeErr: func(info PipeMessage) {
			errListener(info.Output)
		},
		Command:     "ffmpeg",
		CommandArgs: []string{"-i", inputFilePath, "-y", "-progress", "pipe:2", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d", frameDistance, frameHeight), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", filepath.Join(dirPreview, outFile)},
	})
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

// GetVideoInfo Generate file information via ffprobe in JSON and parses it from stout.
func GetVideoInfo(filepath string) (*FFProbeInfo, error) {
	cmd := exec.Command("ffprobe", "-i", filepath, "-show_entries", "format=bit_rate,size,duration", "-show_entries", "stream=r_frame_rate,width,height,nb_read_packets", "-v", "error", "-select_streams", "v:0", "-count_packets", "-of", "default=noprint_wrappers=1", "-print_format", "json")
	stdout, err := cmd.CombinedOutput()
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
		BitRate:     0,
		Size:        0,
		Height:      0,
		Width:       0,
		Duration:    0,
		Fps:         0,
		PacketCount: 0,
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

	packets, err := strconv.ParseUint(parsed.Streams[0].PacketCount, 10, 64)
	if err != nil {
		return info, err
	}
	info.PacketCount = packets

	info.Width = parsed.Streams[0].Width
	info.Height = parsed.Streams[0].Height

	return info, nil
}

func MergeVideos(outputListener func(string), absoluteMergeTextfile, absoluteOutputFilepath string) error {
	log.Println("---------------------------------------------- Merge Job ----------------------------------------------")
	log.Println(absoluteMergeTextfile)
	log.Println(absoluteOutputFilepath)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return ExecSync(&ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-hide_banner", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", absoluteMergeTextfile, "-movflags", "faststart", "-codec", "copy", absoluteOutputFilepath},
		OnStart: func(info CommandInfo) {

		},
		OnPipeErr: func(info PipeMessage) {
			outputListener(info.Output)
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

	return ExecSync(&ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-progress", "pipe:2", "-hide_banner", "-loglevel", "error", "-i", absoluteFilepath, "-ss", startIntervals, "-to", endIntervals, "-movflags", "faststart", "-codec", "copy", absoluteOutputFilepath},
		OnStart: func(info CommandInfo) {
			args.OnStart(&info)
		},
		OnPipeErr: func(info PipeMessage) {
			args.OnProgress(info.Output)
		},
	})
}

func ParseFFmpegKVs(text string) map[string]string {
	lines := strings.Split(text, "\n")

	kvs := make(map[string]string)
	for _, line := range lines {
		kv := strings.Split(line, "=")
		if len(kv) > 1 {
			kvs[kv[0]] = kv[1]
		}
	}

	return kvs
}

func CheckVideo(filepath string) error {
	return ExecSync(&ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-v", "error", "-i", filepath, "-f", "null", "-"},
	})
}
