package utils

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
)

type SysInfo struct {
	CpuUsage  []float64 `json:"cpuUsage"`
	DiskTotal uint64    `json:"diskTotal"`
	DiskFree  uint64    `json:"diskFree"`
	Network   *DevStat  `json:"network"`
}

func Info(path, networkDev string, measureSeconds uint64) (*SysInfo, error) {

	usage, err := disk.Usage(path)
	if err != nil {
		return nil, err
	}

	cpuUsage, err := cpu.Percent(time.Duration(measureSeconds) * time.Second, true)
	if err != nil {
		return nil, err
	}

	network := Measure(networkDev, measureSeconds)

	info := &SysInfo{
		CpuUsage:  cpuUsage,
		DiskFree:  usage.Free,
		DiskTotal: usage.Total,
		Network:   network,
	}

	return info, nil
}
