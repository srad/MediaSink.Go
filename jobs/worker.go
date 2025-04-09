package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/srad/mediasink/ent"
	"github.com/srad/mediasink/network"
	"github.com/srad/mediasink/services"
	"log"
	"sync"
	"time"
)

var (
	registryMu sync.RWMutex
)

type Job struct {
	ID      string     `json:"id"`
	Type    string     `json:"type"`
	Payload JobPayload `json:"payload"`
}

// ... (Plugin loading logic: loadPlugins, pluginRegistry, registryMu remains the same)

// ... (EntQueue struct and methods defined above or in separate file)
// ... (mapEntJobToJob helper function)

// --- Worker Function ---
func Worker(ctx context.Context, id int, queue *EntQueue, wg *sync.WaitGroup, client *ent.Client) {
	defer wg.Done()
	log.Printf("Worker %d started", id)
	for {
		// --- Dequeue Logic (remains the same) ---
		job, err := queue.Dequeue(ctx)
		if err != nil {
			// ... handle dequeue errors, check ctx.Done(), return/continue ...
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || err.Error() == "queue closed" {
				log.Printf("Worker %d stopping gracefully: %v", id, err)
				return // Exit worker loop
			}
			log.Printf("Worker %d WARN: Unexpected error during dequeue: %v", id, err)
			time.Sleep(1 * time.Second)
			continue
		}

		jobUUID, _ := uuid.Parse(job.ID)

		// --- Dispatch Logic (remains the same) ---
		registryMu.RLock()
		handler, found := pluginRegistry[job.Type]
		registryMu.RUnlock()

		if !found {
			// ... handle not found: log, mark failed, continue ...
			log.Printf("Worker %d ERROR: No handler found for job type '%s' (Job ID: %s). Marking as failed.", id, job.Type, job.ID)
			failureErr := fmt.Errorf("no plugin handler registered for type '%s'", job.Type)
			_ = queue.MarkFailed(context.Background(), job.ID, failureErr) // Use appropriate queue type
			continue
		}

		// --- Progress Handling & Execution ---
		log.Printf("Worker %d executing job %s (Type: %s)", id, job.ID, job.Type)

		network.BroadCastClients(network.JobStartEvent, services.JobMessage[*any]{Job: job})

		// 1. Create a buffered progress channel for this job execution
		progressChan := make(chan ProgressInfo, 10) // Buffer size can be adjusted

		// 2. Launch a goroutine to consume progress updates
		var progressWg sync.WaitGroup
		progressWg.Add(1)
		go func() {
			defer progressWg.Done()
			log.Printf("[Progress Consumer %d] Started for Job %s", id, job.ID)
			for update := range progressChan { // Reads until the channel is closed by the plugin
				// --- Process the progress update ---
				log.Printf("PROGRESS Job %s: %.1f%% - %s", job.ID, update.Percentage, update.CurrentStep)
				if update.Details != nil {
					log.Printf("  Details: %+v", update.Details)
					network.BroadCastClients(network.JobProgressEvent, services.JobMessage[ProgressInfo]{
						Job:  job,
						Data: update,
					})
					client.Job.
						UpdateOneID(jobUUID).
						SetDetails(*update.Details).
						Save(ctx)
				}
			}
			log.Printf("[Progress Consumer %d] Finished for Job %s (Channel closed)", id, job.ID)
		}()

		// 3. Execute the job, passing the progress channel
		jobCtx, cancelJob := context.WithTimeout(ctx, 90*time.Second) // Job execution timeout
		var execErr error
		var execWg sync.WaitGroup // Wait group to ensure Execute finishes before proceeding
		execWg.Add(1)

		go func() {
			defer execWg.Done()
			// The plugin's Execute method is now responsible for closing progressChan
			execErr = handler.Execute(jobCtx, job.Payload, progressChan)
		}()

		// 4. Wait for the Execute method to complete
		execWg.Wait()
		cancelJob() // Clean up context resources

		// 5. Ensure the progress consumer goroutine has finished (optional but good practice)
		// This ensures all buffered messages sent *before* Execute returned are processed.
		// Note: The progress consumer exits when progressChan is closed by the plugin's defer statement.
		progressWg.Wait()

		// --- Update Job Status in DB (remains the same) ---
		updateCtx, cancelUpdate := context.WithTimeout(context.Background(), 5*time.Second)
		if execErr != nil {
			log.Printf("Worker %d ERROR executing job %s (Type: %s): %v", id, job.ID, execErr)
			_ = queue.MarkFailed(updateCtx, job.ID, execErr) // Use appropriate queue type
			network.BroadCastClients(network.JobErrorEvent, services.JobMessage[*any]{Job: job})
		} else {
			log.Printf("Worker %d finished job %s (Type: %s) successfully", id, job.ID)
			_ = queue.MarkDone(updateCtx, job.ID) // Use appropriate queue type
			network.BroadCastClients(network.JobDoneEvent, services.JobMessage[*any]{Job: job})
		}
		cancelUpdate()
	}
}
