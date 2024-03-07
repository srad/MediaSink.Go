package helpers

// This code is based on: https://github.com/cs8425/NetTop

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type NetTop struct {
	delta     *NetStat
	last      *NetStat
	t0        time.Time
	dt        time.Duration
	Interface string
}

type NetStat struct {
	Dev  []string
	Stat map[string]*DevStat
}

type DevStat struct {
	Name             string  `json:"name"`
	ReceivedBytes    float64 `json:"receivedBytes"`
	TransmittedBytes float64 `json:"transmittedBytes"`
	MeasureSeconds   uint64  `json:"measureSeconds"`
}

func GetInfo(dev string) *DevStat {
	lines, _ := ReadLines("/proc/net/dev")

	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		if key == dev {
			value := strings.Fields(strings.TrimSpace(fields[1]))

			c := new(DevStat)
			// c := DevStat{}
			c.Name = key
			r, err := strconv.ParseInt(value[0], 10, 64)
			if err != nil {
				break
			}
			c.ReceivedBytes = float64(r)

			t, err := strconv.ParseInt(value[8], 10, 64)
			if err != nil {
				break
			}
			c.TransmittedBytes = float64(t)

			return c
		}
	}

	return nil
}

func ReadLines(filename string) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string

	r := bufio.NewReader(f)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}
	return ret, nil
}

type NetTopInfo struct {
	DRx string `json:"dRx"`
	DTx string `json:"dTx"`
}
