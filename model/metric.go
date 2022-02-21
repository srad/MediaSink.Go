package model

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var (
	quitMetrics = make(chan bool)
)

type SysInfo struct {
	CpuInfo  CPUInfo  `json:"cpuInfo"`
	DiskInfo DiskInfo `json:"diskInfo"`
	NetInfo  NetInfo  `json:"netInfo"`
}

type NetInfo struct {
	Dev           string    `json:"dev"`
	TransmitBytes uint64    `json:"transmitBytes"`
	ReceiveBytes  uint64    `json:"receiveBytes"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (NetInfo) TableName() string {
	return "network_metrics"
}

type DiskInfo struct {
	Size  string `json:"size"`
	Used  string `json:"used"`
	Avail string `json:"avail"`
	Pcent string `json:"pcent"`
}

type CPULoad struct {
	CPU       string    `json:"cpu"`
	Load      float64   `json:"load"`
	CreatedAt time.Time `json:"createdAt"`
}

func (CPULoad) TableName() string {
	return "cpu_metrics"
}

type CPUInfo struct {
	LoadCpu []CPULoad `json:"loadCpu"`
}

type CPUMeasure struct {
	CPU   string
	Idle  float64
	Total float64
}

func GetNetworkMeasure() (*[]NetInfo, error) {
	var info *[]NetInfo
	if err := Db.Model(&NetInfo{}).
		Order("created_at asc").
		Find(&info).Error; err != nil {
		return nil, err
	}

	return info, nil
}

func GetCpuMeasure() (*[]CPULoad, error) {
	var load *[]CPULoad
	if err := Db.Model(&CPULoad{}).
		Order("created_at asc").
		Find(&load).Error; err != nil {
		return nil, err
	}

	return load, nil
}

func StartMetrics(networkDev string) {
	go trackCpu()
	go trackNetwork(networkDev)
}

func trackCpu() {
	for {
		select {
		case <-quitMetrics:
			log.Println("[trackCpu] stopped")
			return
		default:
			// sleeps automatically
			cpu, err := cpuUsage(30)
			if err != nil {
				log.Printf("[trackCpu] Error reasing cpu: %v", err)
				return
			}

			if err := Db.Model(&CPULoad{}).Create(cpu.LoadCpu).Error; err != nil {
				log.Printf("[trackCpu] Error saving metric: %v", err)
			}
		}
	}
}

func trackNetwork(networkDev string) {
	for {
		select {
		case <-quitMetrics:
			log.Println("[trackNetwork] stopped")
			return
		default:
			netInfo, err := netInfo(networkDev, 15)
			if err != nil {
				log.Println("[trackNetwork] stopped")
				return
			}
			if err := Db.Model(&NetInfo{}).Create(netInfo).Error; err != nil {
				log.Printf("[trackCpu] Error saving metric: %v", err)
			}
		}
	}
}

func cpuUsage(waitSeconds uint64) (*CPUInfo, error) {
	cpu := CPUInfo{}

	measure0, err := cpuMeasures()
	if err != nil {
		return nil, err
	}

	time.Sleep(time.Duration(waitSeconds) * time.Second)

	measure1, err := cpuMeasures()
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(measure1); i++ {
		dIdle := measure1[i].Idle - measure0[i].Idle
		dTotal := measure1[i].Total - measure0[i].Total
		cpu.LoadCpu = append(cpu.LoadCpu, CPULoad{CPU: measure1[i].CPU, Load: 1.0 - (dIdle / dTotal)})
	}

	return &cpu, nil
}

func cpuMeasures() ([]CPUMeasure, error) {
	// Documentation of values: https://www.linuxhowtos.org/System/procstat.htm
	//The very first "cpu" line aggregates the numbers in all of the other "cpuN" lines.
	//
	//These numbers identify the amount of time the CPU has spent performing different kinds of work. Time units are in USER_HZ or Jiffies (typically hundredths of a second).
	//
	//The meanings of the columns are as follows, from left to right:
	//
	//user: normal processes executing in user mode
	//nice: niced processes executing in user mode
	//system: processes executing in kernel mode
	//idle: twiddling thumbs
	//iowait: waiting for I/O to complete
	//irq: servicing interrupts
	//softirq: servicing softirqs

	out, err := ioutil.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}

	rows := strings.Split(string(out), "\n")
	var measures []CPUMeasure

	if err != nil {
		return nil, err
	}
	// i := 1, skip first row, calculate individual cpus
	for _, row := range rows {
		cols := strings.Fields(row)
		// Skip empty rows
		if len(cols) == 0 {
			continue
		}
		if strings.Contains(cols[0], "cpu") {
			idle, total, err := parseCpuStats(cols)
			if err != nil {
				return nil, err
			}
			measures = append(measures, CPUMeasure{CPU: cols[0], Idle: idle, Total: total})
		}
	}

	return measures, nil
}

// OUTPUT: | CPUx | user | nice | system | idle | iowait | irq | softirq |
//            0      1      2       3       4       5       6       7
func parseCpuStats(cols []string) (float64, float64, error) {
	var vals []uint64
	sum := uint64(0)

	// skip first column, since "cpux"
	for _, col := range cols[1:] {
		n, err := strconv.ParseUint(col, 10, 64)
		if err != nil {
			return 0, 0, err
		}
		vals = append(vals, n)
		sum += n
	}

	// Source: https://rosettacode.org/wiki/Linux_CPU_utilization
	return float64(vals[3]), float64(sum), nil
}

func DiskUsage(path string) (*DiskInfo, error) {
	// df -h -BG --output=used,avail,pcent / | tail -n1
	out, err := exe("df", "-h", "-BG", "--output=size,used,avail,pcent", path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	parts := strings.Fields(lines[1])

	return &DiskInfo{Size: parts[0], Used: parts[1], Avail: parts[2], Pcent: parts[3]}, nil
}

func netInfo(networkDev string, measureSeconds uint64) (*NetInfo, error) {
	startNet, err := networkTraffic(networkDev)
	if err != nil {
		return nil, err
	}
	time.Sleep(time.Duration(measureSeconds) * time.Second)

	endNet, err := networkTraffic(networkDev)
	if err != nil {
		return nil, err
	}

	diffNet := &NetInfo{
		Dev:           networkDev,
		ReceiveBytes:  endNet.ReceiveBytes - startNet.ReceiveBytes,
		TransmitBytes: endNet.TransmitBytes - startNet.TransmitBytes,
	}

	return diffNet, nil
}

func Info(path, networkDev string, measureSeconds uint64) (*SysInfo, error) {
	disk, err := DiskUsage(path)
	if err != nil {
		return nil, err
	}

	cpuUsage, err := cpuUsage(measureSeconds)
	if err != nil {
		return nil, err
	}

	diffNet, err := netInfo(networkDev, measureSeconds)
	if err != nil {
		return nil, err
	}

	info := &SysInfo{
		CpuInfo:  *cpuUsage,
		DiskInfo: *disk,
		NetInfo:  *diffNet,
	}

	return info, nil
}

func networkTraffic(device string) (*NetInfo, error) {
	out, err := ioutil.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, err
	}

	dev := strings.ToLower(device)
	rows := strings.Split(string(out), "\n")

	// 1: skip headers
	for _, row := range rows[2:] {
		if strings.Contains(strings.ToLower(row), dev) {
			cols := strings.Fields(row)
			rec, err := strconv.ParseUint(cols[1], 10, 64)
			if err != nil {
				return nil, err
			}
			trans, err := strconv.ParseUint(cols[9], 10, 64)
			if err != nil {
				return nil, err
			}

			return &NetInfo{ReceiveBytes: rec, TransmitBytes: trans}, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("Device '%s' not found", dev))
}

func exe(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return "", err
	}

	return string(out), err
}
