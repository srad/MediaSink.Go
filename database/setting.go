package database

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"gorm.io/gorm"
)

type Setting struct {
	SettingKey   string `json:"settingKey" gorm:"primaryKey;" extensions:"!x-nullable"`
	SettingValue string `json:"settingValue" gorm:"not null;" extensions:"!x-nullable"`
	SettingType  string `json:"-" gorm:"not null;" extensions:"!x-nullable"`
}

const (
	MinDuration = "min_duration"
	ReqInterval = "req_interval"
)

func InitSettings() error {
	if err := Db.FirstOrCreate(&Setting{SettingKey: MinDuration, SettingValue: "15", SettingType: "int"}, &Setting{SettingKey: MinDuration}).Error; err != nil {
		log.Printf("[Setting] Init error: %s", err.Error())
		return err
	}

	if err := Db.FirstOrCreate(&Setting{SettingKey: ReqInterval, SettingValue: "15", SettingType: "int"}, &Setting{SettingKey: MinDuration}).Error; err != nil {
		log.Printf("[Setting] Init error: %s", err.Error())
		return err
	}

	return nil
}

func GetValue(settingKey string) (interface{}, error) {
	sett := Setting{}

	if err := Db.Table("settings").First(&sett, &Setting{SettingKey: settingKey}).Error; err != nil {
		log.Printf("[GetValue] Error retreiving setting: %s", err.Error())
		return nil, err
	}

	switch sett.SettingType {
	case "int":
		i, err := strconv.Atoi(sett.SettingValue)
		return i, err
	case "string":
		return sett.SettingValue, nil
	case "bool":
		return sett.SettingValue == "true", nil
	}

	return nil, errors.New(fmt.Sprintf("[GetValue] Unknown settings type: %s", sett.SettingType))
}

func (setting *Setting) Save() error {
	if err := Db.Model(&setting).Where("setting_key = ? ", setting.SettingKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err2 := Db.Create(&setting).Error; err2 != nil {
				return err2
			}
		} else {
			log.Printf("[SaveValue] Error retreiving setting: %s", err.Error())
			return err
		}
	}

	return nil
}
