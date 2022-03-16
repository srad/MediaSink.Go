package models

import (
	"fmt"
	"github.com/srad/streamsink/conf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
)

var Db *gorm.DB

func Init() {
	conf.Read()
	db, err := gorm.Open(sqlite.Open(conf.AppCfg.DbFileName), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to connect database")
	}

	//db.LogMode(true)
	Db = db

	migrate()
}

func migrate() {
	// Migrate the schema
	if err := Db.AutoMigrate(&Channel{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Channel: %v", err))
	}
	if err := Db.AutoMigrate(&Recording{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Info: %v", err))
	}
	if err := Db.AutoMigrate(&Job{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Job: %v", err))
	}
	if err := Db.AutoMigrate(&Setting{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Setting: %v", err))
	}

	// No temp tables for SQLite3, just delete on start.
	//if err := db.AutoMigrate(&NetInfo{}); err != nil {
	//	panic(fmt.Sprintf("[Migrate] Error NetInfo: %v", err))
	//} else {
	//	db.Delete(&NetInfo{}, "1=1")
	//}
	//if err := db.AutoMigrate(&CPULoad{}); err != nil {
	//	panic(fmt.Sprintf("[Migrate] Error CPULoad: %v", err))
	//} else {
	//	db.Delete(&CPULoad{}, "1=1")
	//}

	// Update added display_name
	if err := Db.Exec("UPDATE channels SET display_name = CASE WHEN display_name is null or display_name = '' THEN channel_name ELSE display_name END;").Error; err != nil {
		log.Panicf("[Migrate] Error updating empty display_name: %s", err.Error())
	}

	a := Setting{SettingKey: MinDuration, SettingValue: "15", SettingType: "int"}
	a.Save()
	b := Setting{SettingKey: ReqInterval, SettingValue: "2", SettingType: "int"}
	b.Save()

	InitSettings()
}
