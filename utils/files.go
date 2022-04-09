package utils

import (
	"fmt"
	"time"
)

func CreateRecordingName(channelName string) (string, time.Time) {
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	return fmt.Sprintf("%s_%s.mp4", channelName, stamp), now
}

func CreateMp3Filename(channelName string) (string, time.Time) {
	now := time.Now()
	stamp := now.Format("2006_01_02_15_04_05")
	return fmt.Sprintf("%s_%s.mp3", channelName, stamp), now
}
