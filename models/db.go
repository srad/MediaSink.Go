package models

import (
	"fmt"
	"github.com/srad/streamsink/conf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Db *gorm.DB

func Init() *gorm.DB {
	conf.Read()
	db, err := gorm.Open(sqlite.Open(conf.AppCfg.DbFileName), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	if err := db.AutoMigrate(&Channel{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Channel: %v", err))
	}
	if err := db.AutoMigrate(&Recording{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Info: %v", err))
	}
	if err := db.AutoMigrate(&Job{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Job: %v", err))
	}
	//db.LogMode(true)
	Db = db
	return Db
}
