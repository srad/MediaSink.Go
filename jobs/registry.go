package jobs

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var ( // --- Plugin Registry ---
	pluginRegistry = make(map[string]JobWorker)
)

func LoadPlugins(dir string) error {
	pluginDir := "../plugins"
	numWorkers := 4
	dbPath := "./jobs_ent.db" // Database file path

	// Initialize logger
	log.SetFlags(log.LstdFlags | log.Lshortfile) // Include file/line number

	// --- Plugin Loading ---
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		log.Fatalf("Failed to create plugin directory: %v", err)
	}
	if err := LoadPlugins(pluginDir); err != nil {
		log.Printf("Warning: Error during plugin loading: %v", err)
	}
	if len(pluginRegistry) == 0 {
		log.Println("Warning: No plugins were loaded successfully.")
	}

	// --- Setup Ent Queue ---
	queue, err := NewEntQueue(dbPath) // Use the Ent version
	if err != nil {
		log.Fatalf("Failed to initialize Ent SQLite queue: %v", err)
	}
	defer func() {
		if err := queue.Close(); err != nil {
			log.Printf("Error closing queue: %v", err)
		}
	}()

	// --- Setup Worker Pool ---
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	log.Printf("Starting %d workers...", numWorkers)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go Worker(ctx, i+1, queue, &wg) // Pass the Ent queue
	}

	// --- Example Enqueue Jobs ---
	log.Println("Enqueuing example jobs...")
	err = EnqueueJob(queue, "send_email", map[string]string{"to": "ent@example.com", "subject": "Hello Ent!"})
	if err != nil {
		log.Printf("Failed to enqueue email job: %v", err)
	}
	err = EnqueueJob(queue, "generate_report", map[string]interface{}{"report_id": 1011, "format": "json"})
	if err != nil {
		log.Printf("Failed to enqueue report job: %v", err)
	}

	// --- Wait for shutdown signal ---
	log.Println("Running job system... Press Ctrl+C to stop.")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	// --- Graceful Shutdown ---
	log.Println("Shutting down...")
	cancel() // Signal workers via context

	log.Println("Waiting for workers to finish...")
	wg.Wait() // Wait for workers

	// Queue is closed by defer statement

	log.Println("Shutdown complete.")

	return nil
}
