package database

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/streamsink/conf"
	"gorm.io/driver/mysql"
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
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			//ParameterizedQueries:      true,         // Don't include params in the SQL log
			Colorful: true, // Disable color
		},
	)

	// Choose driver.
	var dialector gorm.Dialector
	switch os.Getenv("DB_ADAPTER") {
	case "mysql":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", os.Getenv("DB_HOST"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"), os.Getenv("DB_PORT"))
		dialector = mysql.New(mysql.Config{DSN: dsn})
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Europe/Berlin", os.Getenv("DB_HOST"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_NAME"), os.Getenv("DB_PORT"))
		dialector = postgres.New(postgres.Config{DSN: dsn})
	default:
		// SQLite3
		dialector = sqlite.Open(cfg.DbFileName)
	}

	/// Open and assign database.
	config := &gorm.Config{
		Logger:                                   newLogger,
		DisableForeignKeyConstraintWhenMigrating: true,
	}
	db, err := gorm.Open(dialector, config)
	if err != nil {
		panic("failed to connect models")
	}
	Db = db

	migrate()
}

func migrate() {
	// Migrate the schema
	if err := Db.AutoMigrate(&User{}); err != nil {
		panic(fmt.Sprintf("[Migrate] Error user: %s", err))
	}
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
