package helpers

import "testing"

func TestCpuUsage(t *testing.T) {
	_, err := CpuUsage(2)
	if err != nil {
		t.Fatalf("Error calculating cpu load: %v", err)
	}
}

func TestDiskUsage(t *testing.T) {
	_, err := DiskUsage("/")
	if err != nil {
		t.Fatalf("Error getting disk usage: %v", err)
	}
}
