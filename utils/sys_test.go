package utils

import "testing"

func TestCpuUsage(t *testing.T) {
	c, err := CpuUsage(2)
	if err != nil {
		t.Fatalf("Error calculating cpu load: %v", err)
	}
	t.Log(c)
}

func TestDiskUsage(t *testing.T) {
	_, err := DiskUsage("/")
	if err != nil {
		t.Fatalf("Error getting disk usage: %v", err)
	}
}
