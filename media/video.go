package media

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/srad/streamsink/utils"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/srad/streamsink/conf"
)

var (
	_, b, _, _  = runtime.Caller(0)
	basePath    = filepath.Dir(b)
	threadCount = uint(float32(runtime.NumCPU() / 2))
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

const (
	winFont           = "C\\\\:/Windows/Fonts/DMMono-Regular.ttf"
	linuxFont         = "/usr/share/fonts/truetype/DMMono-Regular.ttf"
	ffmpegProgressUrl = "tcp://localhost:3333"
	FrameCount        = 96
)

func CheckVideo(filepath string) error {
	return utils.ExecSync("ffmpeg", "-v", "error", "-i", filepath, "-f", "null", "-")
}

func GeneratePreviews(channelName, filename string) error {
	inputPath := filepath.Join(conf.AppCfg.RecordingsAbsolutePath, channelName, filename)

	log.Println("---------------------------------------------- Preview Job ----------------------------------------------")
	log.Println(inputPath)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return ExtractFrames(inputPath, conf.AbsoluteDataPath(channelName), FrameCount, 128, 256)
}

func MergeVideos(absoluteMergeTextfile, absoluteOutputFilepath string) error {
	log.Println("---------------------------------------------- Merge Job ----------------------------------------------")
	log.Println(absoluteMergeTextfile)
	log.Println(absoluteOutputFilepath)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return utils.ExecSync("ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", absoluteMergeTextfile, "-codec", "copy", absoluteOutputFilepath)
}

func CutVideo(absoluteFilepath, absoluteOutputFilepath, startInvervals, endIntervals string) error {
	log.Println("---------------------------------------------- Cutting Job ----------------------------------------------")
	log.Println(absoluteFilepath)
	log.Println(absoluteOutputFilepath)
	log.Println(startInvervals)
	log.Println(endIntervals)
	log.Println("---------------------------------------------------------------------------------------------------------")

	return utils.ExecSync("ffmpeg", "-i", absoluteFilepath, "-ss", startInvervals, "-to", endIntervals, "-codec", "copy", absoluteOutputFilepath)
}

func ExtractFrames(inputPath, outputDir string, extractCount int, frameHeight, videoHeight uint) error {
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

	err = createPreviewStripe(outputDir, filename+".jpg", inputPath, frameDistance, frameHeight, info.Fps)
	if err != nil {
		return errors.New(fmt.Sprintf("error generating stripe for '%s': %v", inputPath, err))
	}

	err = createPreviewVideo(outputDir, filename+".mp4", inputPath, frameDistance, videoHeight, info.Fps)
	if err != nil {
		return errors.New(fmt.Sprintf("error generating preview video for '%s': %v", inputPath, err))
	}

	return nil
}

func getFontPath() string {
	if runtime.GOOS == "windows" {
		return winFont
	}
	return linuxFont
}

func createPreviewStripe(outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) error {
	dir := filepath.Join(outputDir, "stripes")
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}

	return utils.ExecSync("ffmpeg", "-i", absolutePath, "-y", "-progress", ffmpegProgressUrl, "-frames:v", "1", "-q:v", "0", "-threads", fmt.Sprint(threadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,tile=%dx1", frameDistance, frameHeight, getFontPath(), fps, FrameCount), "-hide_banner", "-loglevel", "error", "-stats", "-vsync", "vfr", filepath.Join(dir, outFile))
}

func createPreviewVideo(outputDir, outFile, absolutePath string, frameDistance, frameHeight uint, fps float64) error {
	dir := filepath.Join(outputDir, "videos")
	err := os.MkdirAll(dir, 0777)
	if err != nil {
		return err
	}

	return utils.ExecSync("ffmpeg", "-i", absolutePath, "-y", "-progress", ffmpegProgressUrl, "-q:v", "0", "-threads", fmt.Sprint(threadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,setpts=(7/2)*N/TB", frameDistance, frameHeight, getFontPath(), fps), "-hide_banner", "-loglevel", "error", "-stats", "-vsync", "vfr", filepath.Join(dir, outFile))
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
	err := utils.ExecSync("ffmpeg", "-y", "-hide_banner", "-loglevel", "error", "-i", input, "-r", "1", "-vf", "scale="+height+":-1", "-q:v", "2", "-frames:v", "1", output)

	if err != nil {
		log.Printf("[Recorder] Error extracting frame: %v", err.Error())
		return nil
	}

	return nil
}

// This requires an entire video passthrough
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
	// ffprobe -i ... -show_entries format=r_frame_rate,bit_rate,size,duration -v error -select_streams v:0 -of default=noprint_wrappers=1 -print_format json
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
