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
	CPUInfo  CPUInfo  `json:"cpuInfo" extensions:"!x-nullable"`
	DiskInfo DiskInfo `json:"diskInfo" extensions:"!x-nullable"`
	NetInfo  NetInfo  `json:"netInfo" extensions:"!x-nullable"`
}

type DiskInfo struct {
	SizeFormattedGb  int `json:"sizeFormattedGb" extensions:"!x-nullable"`
	UsedFormattedGb  int `json:"usedFormattedGb" extensions:"!x-nullable"`
	AvailFormattedGb int `json:"availFormattedGb" extensions:"!x-nullable"`
	Pcent            int `json:"pcent" extensions:"!x-nullable"`
}

type NetInfo struct {
	Dev           string    `json:"dev" extensions:"!x-nullable"`
	TransmitBytes uint64    `json:"transmitBytes" extensions:"!x-nullable"`
	ReceiveBytes  uint64    `json:"receiveBytes" extensions:"!x-nullable"`
	CreatedAt     time.Time `json:"createdAt" extensions:"!x-nullable"`
}

func (NetInfo) TableName() string {
	return "network_metrics"
}

type CPULoad struct {
	CPU       string    `json:"cpu" extensions:"!x-nullable"`
	Load      float64   `json:"load" extensions:"!x-nullable"`
	CreatedAt time.Time `json:"createdAt" extensions:"!x-nullable"`
}

type CPUInfo struct {
	LoadCPU []CPULoad `json:"loadCpu" extensions:"!x-nullable"`
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

	timestamp := time.Now()
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
		cpu.LoadCPU = append(cpu.LoadCPU, CPULoad{CPU: measure1[i].CPU, Load: 1.0 - (dIdle / dTotal), CreatedAt: timestamp})
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

	size, err1 := ParseNumbers(parts[0])
	used, err2 := ParseNumbers(parts[1])
	avail, err3 := ParseNumbers(parts[2])
	pcent, err4 := ParseNumbers(parts[3])

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return nil, errors.Join(err1, err2, err3, err4)
	}

	return &DiskInfo{SizeFormattedGb: size, UsedFormattedGb: used, AvailFormattedGb: avail, Pcent: pcent}, nil
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
