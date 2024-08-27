package database

import (
	"github.com/srad/streamsink/helpers"
	"path/filepath"
	"runtime"
	"testing"
)

var (
	_, filestring, _, _ = runtime.Caller(0)
	basefolder          = filepath.Dir(filestring)
	file                = filepath.Join(basefolder, "..", "assets", "test.mp4")
	outputPath          = filepath.Join(basefolder, "..", "assets")
	video               = helpers.Video{FilePath: file}
)

func TestGetFrameCount(t *testing.T) {
	video := &helpers.Video{FilePath: file}
	count, err := video.GetFrameCount()
	if err != nil {
		t.Errorf("error computing framecount: %v", err)
	}

	if count != 1445 {
		t.Errorf("Unexpected frame count")
	}
}

func TestGetVideoInfo(t *testing.T) {
	info, err := video.GetVideoInfo()
	if err != nil {
		t.Errorf("error when getting video duration: %v", err)
	}

	if info.BitRate != 699297 {
		t.Errorf("BitRate wrong: %d", info.BitRate)
	}
	if info.Size != 5251725 {
		t.Errorf("SizeFormattedGb wrong: %d", info.Size)
	}
	if info.Fps != 24.03846153846154 {
		t.Errorf("Fps wrong: %f", info.Fps)
	}
	if info.Duration != 60.08 {
		t.Errorf("Duration wrong: %f", info.Duration)
	}
	if info.Height != 360 {
		t.Errorf("Height wrong: %d", info.Height)
	}
	if info.Width != 640 {
		t.Errorf("Width wrong: %d", info.Width)
	}
	if info.PacketCount != 1445 {
		t.Errorf("Packet count wrong: %d", info.PacketCount)
	}
}
