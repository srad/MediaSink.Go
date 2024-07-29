package models

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/conf"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
)

var Db *gorm.DB

func Init() {
	cfg := conf.Read()

	newLogger := logger.New(
		log.New(),
		logger.Config{
			//SlowThreshold:             time.Second,  // Slow SQL threshold
			LogLevel:                  logger.Error, // Log level
			IgnoreRecordNotFoundError: true,         // Ignore ErrRecordNotFound error for logger
			//ParameterizedQueries:      true,         // Don't include params in the SQL log
			Colorful: true, // Disable color
		},
	)

	// If no db host specified, then create a local sqlite3 models. Mainly for dev env.
	if os.Getenv("DB_HOST") != "" {
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=streamsink port=5432 sslmode=disable TimeZone=Europe/Berlin", os.Getenv("DB_HOST"), os.Getenv("DB_USER"), os.Getenv("DB_PASS"))
		db, err := gorm.Open(postgres.New(postgres.Config{
			DSN: dsn,
			// PreferSimpleProtocol: true, // disables implicit prepared statement usage
		}), &gorm.Config{
			Logger: newLogger,
		})
		if err != nil {
			panic("failed to connect models")
		}
		Db = db
	} else {
		// SQLite3
		log.Infof("Opening models: %s", cfg.DbFileName)
		db, err := gorm.Open(sqlite.Open(cfg.DbFileName), &gorm.Config{
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   newLogger,
		})
		if err != nil {
			panic("failed to connect models")
		}
		Db = db
	}

	migrate()
}

func migrate() {
	// Migrate the schema
	if err := Db.AutoMigrate(&Channel{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Channel: %s", err))
	}
	if err := Db.AutoMigrate(&Recording{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Info: %s", err))
	}
	if err := Db.AutoMigrate(&Job{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Job: %s", err))
	}
	if err := Db.AutoMigrate(&Setting{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error Setting: %s", err))
	}
	InitSettings()
}
