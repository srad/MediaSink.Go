package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/srad/streamsink/conf"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type CuttingJob struct {
	OnStart    func(*CommandInfo)
	OnProgress func(string)
}

type CutArgs struct {
	Starts []string `json:"starts"`
	Ends   []string `json:"ends"`
}

type PreviewJob struct {
	OnStart     func(*CommandInfo)
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

func CreatePreviewStripe(errListener func(string), outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) error {
	dir := filepath.Join(outputDir, conf.StripesFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return ExecSync(&ExecArgs{
		OnPipeErr: func(info PipeMessage) {
			errListener(info.Message)
		},
		Command:     "ffmpeg",
		CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:2", "-frames:v", "1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,tile=%dx1", frameDistance, frameHeight, conf.GetFontPath(), fps, conf.FrameCount), "-hide_banner", "-loglevel", "error", "-stats", "-vsync", "vfr", filepath.Join(dir, outFile)},
	})
}

func CreatePreviewPoster(inputPath, outputDir, filename string) error {
	dir := filepath.Join(outputDir, conf.PostersFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return ExtractFirstFrame(inputPath, conf.FrameWidth, filepath.Join(dir, filename))
}

func CreatePreviewVideo(pipeInfo func(info PipeMessage), outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) error {
	dir := filepath.Join(outputDir, conf.VideosFolder)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	return ExecSync(&ExecArgs{
		OnPipeErr:   pipeInfo,
		Command:     "ffmpeg",
		CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:2", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,setpts=(7/2)*N/TB", frameDistance, frameHeight, conf.GetFontPath(), fps), "-hide_banner", "-loglevel", "error", "-stats", "-vsync", "vfr", "-movflags", "faststart", filepath.Join(dir, outFile)},
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
	err := ExecSync(&ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-y", "-hide_banner", "-loglevel", "error", "-i", input, "-r", "1", "-vf", "scale=" + height + ":-1", "-q:v", "2", "-frames:v", "1", output},
	})

	if err != nil {
		log.Printf("[Recorder] Error extracting frame: %v", err.Error())
		return nil
	}

	return nil
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
	filename := FileNameWithoutExtension(basename)

	if err := CreatePreviewStripe(func(s string) {
		log.Printf("[createPreviewStripe] %s", s)
	}, outputDir, filename+".jpg", inputPath, frameDistance, frameHeight, info.Fps); err != nil {
		return errors.New(fmt.Sprintf("error generating stripe for '%s': %s", inputPath, err.Error()))
	}

	i := 1
	if err := CreatePreviewVideo(func(info PipeMessage) {
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

	return ExecSync(&ExecArgs{
		Command:     "ffmpeg",
		CommandArgs: []string{"-progress", "pipe:2", "-hide_banner", "-loglevel", "error", "-i", absoluteFilepath, "-ss", startIntervals, "-to", endIntervals, "-movflags", "faststart", "-codec", "copy", absoluteOutputFilepath},
		OnStart: func(info CommandInfo) {
			args.OnStart(&info)
		},
		OnPipeErr: func(info PipeMessage) {
			args.OnProgress(info.Message)
		},
	})
}
