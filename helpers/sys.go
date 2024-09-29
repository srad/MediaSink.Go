package helpers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	cmd = make(map[int]*exec.Cmd)
)

type CommandInfo struct {
	Command string
	Pid     int
}

type ExecArgs struct {
	cancel      context.CancelFunc
	OnStart     func(CommandInfo)
	OnPipeOut   func(PipeMessage)
	OnPipeErr   func(PipeMessage)
	Command     string
	CommandArgs []string
}

type PipeMessage struct {
	Output string
	Pid    int
}

type SysInfo struct {
	CPUInfo  CPUInfo  `json:"cpuInfo"`
	DiskInfo DiskInfo `json:"diskInfo"`
	NetInfo  NetInfo  `json:"netInfo"`
}

type DiskInfo struct {
	SizeFormattedGb  string `json:"sizeFormattedGb"`
	UsedFormattedGb  string `json:"usedFormattedGb"`
	AvailFormattedGb string `json:"availFormattedGb"`
	Pcent            string `json:"pcent"`
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

type CPULoad struct {
	CPU       string    `json:"cpu"`
	Load      float64   `json:"load"`
	CreatedAt time.Time `json:"createdAt"`
}

func (CPULoad) TableName() string {
	return "cpu_metrics"
}

type CPUInfo struct {
	LoadCPU []CPULoad `json:"loadCpu"`
}

type CPUMeasure struct {
	CPU   string
	Idle  float64
	Total float64
}

func (execArgs *ExecArgs) ToString() string {
	return fmt.Sprintf("%s %s", execArgs.Command, strings.Join(execArgs.CommandArgs, " "))
}

// ExecSync See: https://stackoverflow.com/questions/10385551/get-exit-code-go
func ExecSync(execArgs *ExecArgs) error {
	c := exec.Command(execArgs.Command, execArgs.CommandArgs...)
	log.Infof("Executing: %s", execArgs.ToString())

	// stdout, _ := cmd.StdoutPipe()
	stout, _ := c.StdoutPipe()
	sterr, _ := c.StderrPipe()

	if err := c.Start(); err != nil {
		log.Infof("cmd.Start: %s", err)
		return err
	}

	pid := c.Process.Pid
	cmd[pid] = c
	defer delete(cmd, pid)

	if execArgs.OnStart != nil {
		execArgs.OnStart(CommandInfo{Pid: pid, Command: execArgs.ToString()})
	}

	// Wait group to synchronize goroutines
	var wg sync.WaitGroup

	// Function to read from a given pipe
	readPipe := func(pipe io.ReadCloser, pipeName string) {
		defer wg.Done() // Notify the wait group when done
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			if pipeName == "stdout" && execArgs.OnPipeOut != nil {
				execArgs.OnPipeOut(PipeMessage{Output: scanner.Text(), Pid: pid})
			} else if pipeName == "stderr" && execArgs.OnPipeErr != nil {
				execArgs.OnPipeErr(PipeMessage{Output: scanner.Text(), Pid: pid})
			}
		}
	}

	// Add two goroutines to the wait group (for stdout and stderr)
	wg.Add(2)

	// Start a goroutine to read stdout
	go readPipe(stout, "stdout")

	// Start a goroutine to read stderr
	go readPipe(sterr, "stderr")

	// Wait for the goroutines to finish
	wg.Wait()

	// First check if process still exists, could have been killed in the meantime.
	if _, ok := cmd[pid]; ok {
		if err := cmd[pid].Wait(); err != nil {
			var exiterr *exec.ExitError
			if errors.As(err, &exiterr) {
				// The program has exited with an exit code != 0

				// This works on both Unix and Windows. Although package
				// syscall is generally platform dependent, WaitStatus is
				// defined for both Unix and Windows and in both cases has
				// an ExitStatus() method with the same signature.
				if _, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					return err
					// return status.ExitStatus()
				}
			}
			return err
		}
	}

	return nil
}

func Interrupt(pid int) error {
	if c, ok := cmd[pid]; ok {
		err := c.Process.Signal(syscall.SIGINT)
		delete(cmd, pid)
		return err
	}
	return nil
}

func CPUUsage(waitSeconds uint64) (*CPUInfo, error) {
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
		cpu.LoadCPU = append(cpu.LoadCPU, CPULoad{CPU: measure1[i].CPU, Load: 1.0 - (dIdle / dTotal)})
	}

	return &cpu, nil
}

func cpuMeasures() ([]CPUMeasure, error) {
	// Documentation of values: https://www.linuxhowtos.org/System/procstat.htm
	// The very first "cpu" line aggregates the numbers in all the other "cpuN" lines.
	//
	// These numbers identify the amount of time the CPU has spent performing different kinds of work. Time units are in USER_HZ or Jiffies (typically hundredths of a second).
	//
	// The meanings of the columns are as follows, from left to right:
	//
	// user: normal processes executing in user mode
	// nice: niced processes executing in user mode
	// system: processes executing in kernel mode
	// idle: twiddling thumbs
	// iowait: waiting for I/O to complete
	// irq: servicing interrupts
	// softirq: servicing softirqs

	out, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}

	rows := strings.Split(string(out), "\n")
	var measures []CPUMeasure

	// i := 1, skip first row, calculate individual cpus
	for _, row := range rows {
		cols := strings.Fields(row)
		// Skip empty rows
		if len(cols) == 0 {
			continue
		}
		if strings.Contains(cols[0], "cpu") {
			idle, total, err := parseCPUStats(cols)
			if err != nil {
				return nil, err
			}
			measures = append(measures, CPUMeasure{CPU: cols[0], Idle: idle, Total: total})
		}
	}

	return measures, nil
}

// OUTPUT: | CPUx | user | nice | system | idle | iowait | irq | softirq |
//
//	0      1      2       3       4       5       6       7
func parseCPUStats(cols []string) (float64, float64, error) {
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

	return &DiskInfo{SizeFormattedGb: parts[0], UsedFormattedGb: parts[1], AvailFormattedGb: parts[2], Pcent: parts[3]}, nil
}

func NetMeasure(networkDev string, measureSeconds uint64) (*NetInfo, error) {
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

	cpuUsage, err := CPUUsage(measureSeconds)
	if err != nil {
		return nil, err
	}

	diffNet, err := NetMeasure(networkDev, measureSeconds)
	if err != nil {
		return nil, err
	}

	info := &SysInfo{
		CPUInfo:  *cpuUsage,
		DiskInfo: *disk,
		NetInfo:  *diffNet,
	}

	return info, nil
}

func networkTraffic(device string) (*NetInfo, error) {
	out, err := os.ReadFile("/proc/net/dev")
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

	return nil, fmt.Errorf("device '%s' not found", dev)
}

func exe(cmd string, args ...string) (string, error) {
	out, err := exec.Command(cmd, args...).Output()
	if err != nil {
		return "", err
	}

	return string(out), err
}
