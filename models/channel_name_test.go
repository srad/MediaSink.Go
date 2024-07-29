package models

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

func mockEnvVars() {
	//DbFileName:             getConfString("db.filename", "DB_FILENAME"),
	//RecordingsAbsolutePath: getConfString("dirs.recordings", "REC_PATH"),
	//DataPath:               getConfString("dirs.data", "DATA_DIR"),
	//DataDisk:               getConfString("sys.disk", "DATA_DISK"),
	//NetworkDev:             getConfString("sys.network", "NET_ADAPTER"),
	os.Setenv("REC_PATH", "/recordings")
	os.Setenv("DATA_DIR", ".previews")
}

func TestRelativeDataPath(t *testing.T) {
	mockEnvVars()
	channelName := ChannelName("my_channel")
	expected := "my_channel/.previews"
	fact := channelName.RelativeDataPath()

	if fact != expected {
		t.Errorf("RelativeDataPath() is %s but should be %s", fact, expected)
	}
}

func TestChannelPath(t *testing.T) {
	mockEnvVars()
	channelName := ChannelName("my_channel")
	filename := "my_file.mp4"
	expected := fmt.Sprintf("my_channel/%s", filename)
	fact := channelName.ChannelPath(filename)

	if fact != expected {
		t.Errorf("ChannelPath() %s but should be %s", fact, expected)
	}
}

func TestAbsoluteChannelFilePath(t *testing.T) {
	mockEnvVars()
	channelName := ChannelName("my_channel")
	filename := "my_file.mp4"
	expected := fmt.Sprintf("/recordings/my_channel/%s", filename)
	fact := channelName.AbsoluteChannelFilePath(filename)

	if fact != expected {
		t.Errorf("AbsoluteChannelFilePath() is %s but should be %s", fact, expected)
	}
}

func TestMakeRecordingFilename(t *testing.T) {
	mockEnvVars()
	channelName := ChannelName("my_channel")
	filePattern, _ := regexp.Compile("^[a-z0-9_]+_\\d\\d\\d\\d_\\d\\d_\\d\\d_\\d\\d_\\d\\d_\\d\\d.mp4$")
	expected := fmt.Sprintf("%s_%s.mp4", channelName.String(), filePattern)
	fact, _ := channelName.MakeRecordingFilename()

	if !filePattern.MatchString(fact) {
		t.Errorf("MakeRecordingFilename() is %s but should be %s", fact, expected)
	}
}

func TestCreateMp3Filename(t *testing.T) {
	mockEnvVars()
	channelName := ChannelName("my_channel")
	filePattern, _ := regexp.Compile("^[a-z0-9_]+_\\d\\d\\d\\d_\\d\\d_\\d\\d_\\d\\d_\\d\\d_\\d\\d.mp3")
	expected := fmt.Sprintf("%s_%s.mp3", channelName.String(), filePattern)
	fact, _ := channelName.MakeMp3Filename()

	if !filePattern.MatchString(fact) {
		t.Errorf("MakeRecordingFilename() is %s but should be %s", fact, expected)
	}
}
