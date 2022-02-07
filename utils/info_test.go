package utils

import (
	"testing"
)

func TestCpuUsage(t *testing.T) {
	c, err := cpuUsage(2)
	if err != nil {
		t.Fatalf("Error calculating cpu load: %v", err)
	}
	t.Log(c)
}

func TestDiskUsage(t *testing.T) {
	_, err := diskUsage("/")
	if err != nil {
		t.Fatalf("Error getting disk usage: %v", err)
	}
}
