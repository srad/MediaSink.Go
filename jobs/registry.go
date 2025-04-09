package jobs

import (
    "context"
    "encoding/json"
    "fmt"
    "github.com/google/uuid"
    "github.com/srad/mediasink/ent"
    "github.com/srad/mediasink/utils"
    "log"
    "os"
    "os/signal"
    "path"
    "path/filepath"
    "plugin"
    "sync"
    "syscall"
)

var (
    pluginRegistry = make(map[string]JobWorker)
)

func Start(client *ent.Client) error {
    return _start(path.Join("..", "plugins"), client)
}

func _start(pluginDir string, client *ent.Client) error {
    numWorkers := 1

    // Initialize logger
    log.SetFlags(log.LstdFlags | log.Lshortfile) // Include file/line number

    // --- Plugin Loading ---
    if exists, err := utils.DirExists(pluginDir); !exists || err != nil {
        log.Fatalf("Failed to create plugin directory: %v", err)
    }
    if err := loadPlugins(pluginDir); err != nil {
        log.Fatalf("warning: Error during plugin loading: %v", err)
    }
    if len(pluginRegistry) == 0 {
        log.Println("Warning: No plugins were loaded")
    }

    // --- Setup Ent Queue ---
    queue, err := NewEntQueue(client) // Use the Ent version
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
        go Worker(ctx, i+1, queue, &wg, client) // Pass the Ent queue
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

func getRegisteredTypes() []string {
    registryMu.RLock()
    defer registryMu.RUnlock()
    types := make([]string, 0, len(pluginRegistry))
    for k := range pluginRegistry {
        types = append(types, k)
    }
    return types
}

// --- Load Plugins ---
func loadPlugins(pluginDir string) error {
    registryMu.Lock()
    defer registryMu.Unlock()

    files, err := os.ReadDir(pluginDir)
    if err != nil {
        return fmt.Errorf("failed to read plugin directory %s: %w", pluginDir, err)
    }

    log.Printf("Loading plugins from: %s", pluginDir)
    for _, file := range files {
        if file.IsDir() || filepath.Ext(file.Name()) != ".so" {
            continue
        }

        pluginPath := filepath.Join(pluginDir, file.Name())
        log.Printf("Attempting to load plugin: %s", pluginPath)

        p, err := plugin.Open(pluginPath)
        if err != nil {
            log.Printf("ERROR: Failed to open plugin %s: %v", pluginPath, err)
            continue // Skip faulty plugins
        }

        // Lookup the exported symbol (must match jobspec.PluginSymbol)
        sym, err := p.Lookup(PluginSymbol)
        if err != nil {
            log.Printf("ERROR: Failed to lookup symbol '%s' in plugin %s: %v", PluginSymbol, pluginPath, err)
            continue
        }

        // Type assertion to check if it implements the JobWorker interface
        worker, ok := sym.(JobWorker)
        if !ok {
            // Maybe the symbol is a function returning the worker?
            if newFunc, okFunc := sym.(func() JobWorker); okFunc {
                worker = newFunc()
                ok = true // Mark as ok if function call succeeded
            } else {
                log.Printf("ERROR: Symbol '%s' in plugin %s does not implement jobspec.JobWorker (or func() jobspec.JobWorker): found type %T", PluginSymbol, pluginPath, sym)
                continue
            }
        }

        if !ok { // Still not ok after checking for function
            log.Printf("ERROR: Failed to obtain JobWorker from plugin %s", pluginPath)
            continue
        }

        jobType := worker.Type()
        if _, exists := pluginRegistry[jobType]; exists {
            log.Printf("WARNING: Duplicate plugin found for job type '%s'. Overwriting with %s.", jobType, pluginPath)
        }
        pluginRegistry[jobType] = worker
        log.Printf("Successfully loaded plugin for job type '%s' from %s", jobType, pluginPath)
    }
    log.Printf("Plugin loading complete. Registered types: %v", getRegisteredTypes())
    return nil
}

func EnqueueJob(queue *EntQueue, jobType string, data interface{}) error {
    payloadBytes, err := json.Marshal(data)
    if err != nil {
        return fmt.Errorf("failed to marshal job payload: %w", err)
    }

    job := Job{
        ID:      uuid.NewString(), // Generate UUID string for the internal Job struct
        Type:    jobType,
        Payload: JobPayload(payloadBytes),
    }
    // Use the Enqueue method of the EntQueue
    return queue.Enqueue(job)
}
