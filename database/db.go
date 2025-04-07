package database

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/srad/mediasink/conf"
	"github.com/srad/mediasink/ent"
)

var Client *ent.Client

func Init() {
	cfg := conf.Read()

	// --- 1. Configuration (Get DSN from env/config file) ---
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_journal_mode=WAL&_busy_timeout=5000", cfg.DbFileName)

	// --- 2. Initialize Ent Client ---
	Client, err := ent.Open("sqlite3", dsn)
	if err != nil {
		log.Fatalf("failed opening connection to sqlite: %v", err)

		// Run migrations (optional, can be separate command)
		if err := Client.Schema.Create(context.Background()); err != nil {
			log.Fatalf("failed creating schema resources: %v", err)
		}
		log.Println("Database connection and migration successful.")
	}
}

func Close() {
	Client.Close() // Ensure client is closed on exit
}
