package database

import "time"

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

func (CPULoad) TableName() string {
	return "cpu_metrics"
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
