package helpers

import (
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "os/exec"
    "path"
    "path/filepath"
    "strconv"
    "strings"
    "time"

    "github.com/astaxie/beego/utils"
    log "github.com/sirupsen/logrus"
    "github.com/srad/mediasink/conf"
)

var (
    VideosFolder  = "videos"
    StripesFolder = "stripes"
    CoverFolder   = "posters"
)

// Video Represent a video to which operations can be applied.
type Video struct {
    FilePath string `validate:"required,filepath"`
}

type CuttingJob struct {
    OnStart    func(*CommandInfo)
    OnProgress func(string)
}

type CutArgs struct {
    Starts                []string `json:"starts"`
    Ends                  []string `json:"ends"`
    DeleteAfterCompletion bool     `json:"deleteAfterCut"`
}

type TaskProgress struct {
    Current uint64 `json:"current"`
    Total   uint64 `json:"total"`
    Steps   uint   `json:"steps"`
    Step    uint   `json:"step"`
    Message string `json:"message"`
}

type TaskComplete struct {
    Steps   uint   `json:"steps"`
    Step    uint   `json:"step"`
    Message string `json:"message"`
}

type TaskInfo struct {
    Steps   uint   `json:"steps"`
    Step    uint   `json:"step"`
    Pid     int    `json:"pid"`
    Command string `json:"command"`
    Message string `json:"message"`
}

type VideoConversionArgs struct {
    OnStart    func(info TaskInfo)
    OnProgress func(info TaskProgress)
    OnEnd      func(task TaskComplete)
    OnError    func(error)
    InputPath  string
    OutputPath string
    Filename   string
}

type ProcessInfo struct {
    JobType string
    Frame   uint64
    Total   int
    Raw     string
}

type JSONFFProbeInfo struct {
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
    ChannelName string
    Filename    string
    Filepath    string
    CreatedAt   time.Time
}

type PreviewResult struct {
    FilePath string
    Filename string
}

type PreviewStripeArgs struct {
    OnStart                    func(info CommandInfo)
    OnProgress                 func(TaskProgress)
    OnEnd                      func(task string)
    OnErr                      func(error)
    OutputDir, OutFile         string
    FrameDistance, FrameHeight uint
}

type PreviewVideoArgs struct {
    OnStart                    func(info CommandInfo)
    OnProgress                 func(TaskProgress)
    OnEnd                      func()
    OnErr                      func(error)
    OutputDir, OutFile         string
    FrameDistance, FrameHeight uint
}

type MergeArgs struct {
    OnStart                func(info CommandInfo)
    OnProgress             func(info PipeMessage)
    OnErr                  func(error)
    MergeFileAbsolutePath  string
    AbsoluteOutputFilepath string
}

func (video *Video) createPreviewStripe(arg *PreviewStripeArgs) error {
    dir := filepath.Join(arg.OutputDir, StripesFolder)
    if err := os.MkdirAll(dir, 0777); err != nil {
        return err
    }

    return ExecSync(&ExecArgs{
        OnStart: arg.OnStart,
        OnPipeOut: func(out PipeMessage) {
            kvs := ParseFFmpegKVs(out.Output)

            if frame, ok := kvs["frame"]; ok {
                if value, err := strconv.ParseUint(frame, 10, 64); err == nil && value > 0 {
                    arg.OnProgress(TaskProgress{Current: value})
                }
            }
            if progress, ok := kvs["progress"]; ok {
                if progress == "end" && arg.OnEnd != nil {
                    arg.OnEnd("preview-stripe")
                }
            }
        },
        OnPipeErr: func(pipe PipeMessage) {
            if arg.OnErr != nil {
                arg.OnErr(errors.New(pipe.Output))
            }
        },
        Command:     "ffmpeg",
        CommandArgs: []string{"-i", video.FilePath, "-y", "-progress", "pipe:1", "-frames:v", "1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,tile=%dx1", arg.FrameDistance, arg.FrameHeight, conf.FrameCount), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", filepath.Join(dir, arg.OutFile)},
        // Embed time-code in video
        //CommandArgs: []string{"-i", absolutePath, "-y", "-progress", "pipe:1", "-frames:v", "1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d,drawtext=fontfile=%s: text='%%{pts\\:gmtime\\:0\\:%%H\\\\\\:%%M\\\\\\:%%S}': rate=%f: x=(w-tw)/2: y=h-(2*lh): fontsize=20: fontcolor=white: bordercolor=black: borderw=3: box=0: boxcolor=0x00000000@1,tile=%dx1", frameDistance, frameHeight, conf.GetFontPath(), fps, conf.FrameCount), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", filepath.Join(dir, outFile)},
    })
}

func (video *Video) createPreviewCover(outputDir, filename string) error {
    coverDir := filepath.Join(outputDir, CoverFolder)
    if err := os.MkdirAll(coverDir, 0777); err != nil {
        return err
    }

    path := filepath.Join(coverDir, filename)

    return ExtractFirstFrame(video.FilePath, conf.FrameWidth, path)
}

func (video *Video) createPreviewVideo(args *PreviewVideoArgs) (string, error) {
    dir := filepath.Join(args.OutputDir, VideosFolder)
    if err := os.MkdirAll(dir, 0777); err != nil {
        return "", err
    }

    return dir, ExecSync(&ExecArgs{
        OnStart: args.OnStart,
        OnPipeOut: func(out PipeMessage) {
            kvs := ParseFFmpegKVs(out.Output)

            if frame, ok := kvs["frame"]; ok {
                if value, err := strconv.ParseUint(frame, 10, 64); err == nil && value > 0 {
                    args.OnProgress(TaskProgress{Current: value})
                }
            }
            if progress, ok := kvs["progress"]; ok {
                if progress == "end" && args.OnEnd != nil {
                    args.OnEnd()
                }
            }
        },
        OnPipeErr: func(message PipeMessage) {
            if args.OnErr != nil {
                args.OnErr(errors.New(message.Output))
            }
        },
        Command:     "ffmpeg",
        CommandArgs: []string{"-i", video.FilePath, "-y", "-progress", "pipe:1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d", args.FrameDistance, args.FrameHeight), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", "-movflags", "faststart", filepath.Join(dir, args.OutFile)},
    })
}

func (video *Video) CreatePreviewTimelapse(args *PreviewVideoArgs) (string, error) {
    dir := filepath.Join(args.OutputDir, VideosFolder)
    if err := os.MkdirAll(dir, 0777); err != nil {
        return "", err
    }

    return dir, ExecSync(&ExecArgs{
        OnStart: args.OnStart,
        OnPipeOut: func(out PipeMessage) {
            kvs := ParseFFmpegKVs(out.Output)

            if frame, ok := kvs["frame"]; ok {
                if value, err := strconv.ParseUint(frame, 10, 64); err == nil && value > 0 {
                    args.OnProgress(TaskProgress{Current: value})
                }
            }
            if progress, ok := kvs["progress"]; ok {
                if progress == "end" && args.OnEnd != nil {
                    args.OnEnd()
                }
            }
        },
        OnPipeErr: func(message PipeMessage) {
            if args.OnErr != nil {
                args.OnErr(errors.New(message.Output))
            }
        },
        Command:     "ffmpeg",
        CommandArgs: []string{"-i", video.FilePath, "-y", "-progress", "pipe:1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d", args.FrameDistance, args.FrameHeight), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", "-movflags", "faststart", filepath.Join(dir, args.OutFile)},
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
        return fmt.Errorf("error extracting frame '%s'", err)
    }

    return nil
}

func ConvertVideo(args *VideoConversionArgs, mediaType string) (*ConversionResult, error) {
    input := filepath.Join(args.OutputPath, args.Filename)
    if !utils.FileExists(input) {
        return nil, fmt.Errorf("file '%s' does not exit", input)
    }

    // Might seem redundant, but since we have no dependent types...
    if mediaType == "mp3" {
        mp3Filename := fmt.Sprintf("%s.mp3", FileNameWithoutExtension(args.Filename))
        outputAbsoluteMp3 := filepath.Join(args.OutputPath, mp3Filename)

        result := &ConversionResult{
            Filename:  mp3Filename,
            CreatedAt: time.Now(),
            Filepath:  outputAbsoluteMp3,
        }

        err := ExecSync(&ExecArgs{
            OnPipeErr: func(info PipeMessage) {
                if args.OnError != nil {
                    args.OnError(errors.New(info.Output))
                }
            },
            OnStart: func(info CommandInfo) {
                args.OnStart(TaskInfo{
                    Steps:   3,
                    Pid:     info.Pid,
                    Command: info.Command,
                })
            },
            OnPipeOut: func(message PipeMessage) {
                kvs := ParseFFmpegKVs(message.Output)

                if frame, ok := kvs["frame"]; ok {
                    if value, err := strconv.ParseUint(frame, 10, 64); err == nil {
                        args.OnProgress(TaskProgress{Current: value})
                    }
                }
                if progress, ok := kvs["progress"]; ok {
                    if progress == "end" && args.OnEnd != nil {
                        args.OnEnd(TaskComplete{
                            Steps: 1,
                            Step:  1,
                        })
                    } else {
                        fmt.Println(progress)
                    }
                }
            },
            Command:     "ffmpeg",
            CommandArgs: []string{"-i", input, "-y", "-threads", fmt.Sprint(conf.ThreadCount), "-hide_banner", "-loglevel", "error", "-progress", "pipe:1", "-q:a", "0", "-map", "a", outputAbsoluteMp3},
        })

        return result, err
    }

    // Create new filename
    name := fmt.Sprintf("%s_%s.mp4", FileNameWithoutExtension(args.Filename), mediaType)
    output := filepath.Join(args.OutputPath, name)

    result := &ConversionResult{
        Filename:  name,
        CreatedAt: time.Now(),
        Filepath:  output,
    }

    err := ExecSync(&ExecArgs{
        OnPipeErr: func(info PipeMessage) {
            log.Error(info.Output)
        },
        OnStart: func(info CommandInfo) {
            args.OnStart(TaskInfo{
                Steps:   1,
                Pid:     info.Pid,
                Command: info.Command,
            })
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
        CommandArgs: []string{"-i", input, "-y", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("scale=-1:%s", mediaType), "-hide_banner", "-loglevel", "error", "-progress", "pipe:1", "-movflags", "faststart", "-c:v", "libx264", "-crf", "18", "-preset", "medium", "-c:a", "copy", output},
    })

    return result, err
}

func (video *Video) ExecPreviewStripe(args *VideoConversionArgs, extractCount uint64, frameHeight uint, frameCount uint64) (*PreviewResult, error) {
    frameDistance := uint(float32(frameCount) / float32(extractCount))
    basename := filepath.Base(video.FilePath)
    filename := FileNameWithoutExtension(basename)

    var frame uint64
    err := video.createPreviewStripe(&PreviewStripeArgs{
        OnStart: func(info CommandInfo) {
            args.OnStart(TaskInfo{
                Steps:   2,
                Step:    1,
                Pid:     info.Pid,
                Command: info.Command,
                Message: "Generating stripe",
            })
        },
        OnProgress: func(info TaskProgress) {
            frame++
            args.OnProgress(TaskProgress{
                Current: frame,
                Total:   extractCount,
                Steps:   2,
                Step:    1,
                Message: "Generating stripe",
            })
        },
        OnEnd: func(task string) {
            if args.OnEnd == nil {
                args.OnEnd(TaskComplete{
                    Step:    1,
                    Steps:   2,
                    Message: "Stripe generated",
                })
            }
        },
        OnErr:         args.OnError,
        OutputDir:     args.OutputPath,
        OutFile:       filename + ".jpg",
        FrameDistance: frameDistance,
        FrameHeight:   frameHeight,
    })

    if err != nil {
        return nil, fmt.Errorf("error generating stripe for '%s': %s", video.FilePath, err)
    }

    return &PreviewResult{Filename: args.Filename, FilePath: path.Join(args.OutputPath, filename+".jpg")}, nil
}

func (video Video) ExecPreviewVideo(args *VideoConversionArgs, extractCount uint64, videoHeight uint, frameCount uint64) (*PreviewResult, error) {
    frameDistance := uint(float32(frameCount) / float32(extractCount))
    basename := filepath.Base(video.FilePath)
    filename := FileNameWithoutExtension(basename)

    previewVideoDir, err := video.CreatePreviewTimelapse(&PreviewVideoArgs{
        OnStart: func(info CommandInfo) {
            args.OnStart(TaskInfo{
                Steps:   2,
                Step:    2,
                Pid:     info.Pid,
                Command: info.Command,
                Message: "Generating preview video",
            })
        },
        OnProgress: func(info TaskProgress) {
            args.OnProgress(TaskProgress{
                Current: info.Current,
                Total:   extractCount,
                Steps:   2,
                Step:    2,
                Message: "Generating preview video",
            })
        },
        OnEnd: func() {
            if args.OnEnd == nil {
                args.OnEnd(TaskComplete{
                    Step:    2,
                    Steps:   2,
                    Message: "Video generated",
                })
            }
        },
        OnErr:         args.OnError,
        OutputDir:     args.OutputPath,
        OutFile:       filename + ".mp4",
        FrameDistance: frameDistance,
        FrameHeight:   videoHeight,
    })
    if err != nil {
        return nil, fmt.Errorf("error generating preview video for %s: %s", video.FilePath, err)
    }

    return &PreviewResult{Filename: args.Filename, FilePath: previewVideoDir}, nil
}

func (video Video) ExecPreviewCover(outputPath string) (*PreviewResult, error) {
    basename := filepath.Base(video.FilePath)
    filename := FileNameWithoutExtension(basename)
    file := filename + ".jpg"

    if err := video.createPreviewCover(outputPath, file); err != nil {
        return nil, fmt.Errorf("error generating poster for '%s': %s", video.FilePath, err)
    }

    return &PreviewResult{FilePath: video.FilePath, Filename: file}, nil
}

// CreatePreviewShots Create a separate preview image file, at every frame distance.
//func (video *Video) CreatePreviewShots(errListener func(s string), outputDir string, filename string, frameDistance uint, frameHeight uint, fps float64) (string, error) {
//	dirPreview := filepath.Join(outputDir, conf.ScreensFolder, filename)
//	if err := os.MkdirAll(dirPreview, 0777); err != nil {
//		return dirPreview, err
//	}
//
//	outFile := fmt.Sprintf("%s_%%010d.jpg", filename)
//
//	return dirPreview, ExecSync(&ExecArgs{
//		OnPipeErr: func(info PipeMessage) {
//			errListener(info.Output)
//		},
//		Command:     "ffmpeg",
//		CommandArgs: []string{"-i", video.AbsoluteChannelFilepath, "-y", "-progress", "pipe:1", "-q:v", "0", "-threads", fmt.Sprint(conf.ThreadCount), "-an", "-vf", fmt.Sprintf("select=not(mod(n\\,%d)),scale=-2:%d", frameDistance, frameHeight), "-hide_banner", "-loglevel", "error", "-stats", "-fps_mode", "vfr", filepath.Join(dirPreview, outFile)},
//	})
//}

// GetFrameCount This requires an entire video passthrough
//func (video *Video) GetFrameCount() (uint64, error) {
//	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "stream=nb_read_packets", "-of", "csv=p=0", "-select_streams", "v:0", "-count_packets", video.FilePath)
//	stdout, err := cmd.CombinedOutput()
//	output := strings.TrimSpace(string(stdout))
//
//	if err != nil {
//		return 0, fmt.Errorf("error getting frame count for '%s': %s", video.FilePath, stdout)
//	}
//
//	fps, err := strconv.ParseUint(output, 10, 64)
//	if err != nil {
//		return 0, nil
//	}
//
//	return fps, nil
//}

// GetVideoInfo Generate file information via ffprobe in JSON and parses it from stout.
func (video *Video) GetVideoInfo() (*FFProbeInfo, error) {
    cmd := exec.Command("ffprobe", "-i", video.FilePath, "-show_entries", "format=bit_rate,size,duration", "-show_entries", "stream=r_frame_rate,width,height,nb_read_packets", "-v", "error", "-select_streams", "v:0", "-count_packets", "-of", "default=noprint_wrappers=1", "-print_format", "json")
    stdout, err := cmd.CombinedOutput()
    output := strings.TrimSpace(string(stdout))

    if err != nil {
        return nil, fmt.Errorf("error ffprobe: %s: %s", err, output)
    }

    parsed := &JSONFFProbeInfo{}
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

func MergeVideos(args *MergeArgs) error {
    log.Infoln("---------------------------------------------- Merge Job ----------------------------------------------")
    log.Infoln(args.MergeFileAbsolutePath)
    log.Infoln(args.AbsoluteOutputFilepath)
    log.Infoln("---------------------------------------------------------------------------------------------------------")

    return ExecSync(&ExecArgs{
        Command:     "ffmpeg",
        CommandArgs: []string{"-hide_banner", "-loglevel", "error", "-f", "concat", "-safe", "0", "-i", args.MergeFileAbsolutePath, "-movflags", "faststart", "-codec", "copy", args.AbsoluteOutputFilepath},
        OnStart:     args.OnStart,
        OnPipeErr: func(info PipeMessage) {
            if args.OnErr != nil {
                args.OnErr(errors.New(info.Output))
            }
        },
        OnPipeOut: args.OnProgress,
    })
}

func CutVideo(args *CuttingJob, absoluteFilepath, absoluteOutputFilepath, startIntervals, endIntervals string) error {
    log.Infoln("---------------------------------------------- Cutting Job ----------------------------------------------")
    log.Infoln(absoluteFilepath)
    log.Infoln(absoluteOutputFilepath)
    log.Infoln(startIntervals)
    log.Infoln(endIntervals)
    log.Infoln("---------------------------------------------------------------------------------------------------------")

    return ExecSync(&ExecArgs{
        Command:     "ffmpeg",
        CommandArgs: []string{"-progress", "pipe:1", "-hide_banner", "-loglevel", "error", "-i", absoluteFilepath, "-ss", startIntervals, "-to", endIntervals, "-movflags", "faststart", "-codec", "copy", absoluteOutputFilepath},
        OnStart: func(info CommandInfo) {
            args.OnStart(&info)
        },
        OnPipeErr: func(info PipeMessage) {
            log.Error(info.Output)
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
