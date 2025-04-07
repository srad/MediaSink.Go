package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
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
// Type hint for queue changes to *EntQueue
func Worker(ctx context.Context, id int, queue *EntQueue, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Printf("Worker %d started", id)
	for {
		// Dequeue logic using EntQueue's Dequeue method
		job, err := queue.Dequeue(ctx) // Pass the main context

		if err != nil {
			// Check if the error is due to context cancellation or closed queue
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || err.Error() == "queue closed" {
				log.Printf("Worker %d stopping gracefully: %v", id, err)
				return // Exit worker loop
			}
			// Log other unexpected dequeue errors (EntQueue handles NotFound internally for polling)
			log.Printf("Worker %d WARN: Unexpected error during dequeue: %v", id, err)
			time.Sleep(1 * time.Second) // Avoid busy-looping on persistent errors
			continue
		}

		// --- Dispatch Logic (remains the same) ---
		registryMu.RLock()
		handler, found := pluginRegistry[job.Type]
		registryMu.RUnlock()

		if !found {
			log.Printf("Worker %d ERROR: No handler found for job type '%s' (Job ID: %s). Marking as failed.", id, job.Type, job.ID)
			failureErr := fmt.Errorf("no plugin handler registered for type '%s'", job.Type)
			// Use background context for marking failure
			if markErr := queue.MarkFailed(context.Background(), job.ID, failureErr); markErr != nil {
				log.Printf("Worker %d ERROR: Failed to mark job %s as failed: %v", id, job.ID, markErr)
			}
			continue
		}

		// --- Execution Logic ---
		log.Printf("Worker %d executing job %s (Type: %s)", id, job.ID, job.Type)
		// Execute with timeout
		jobCtx, cancelJob := context.WithTimeout(ctx, 30*time.Second)
		execErr := handler.Execute(jobCtx, job.Payload)
		cancelJob()

		// --- Update Job Status using EntQueue methods ---
		updateCtx, cancelUpdate := context.WithTimeout(context.Background(), 5*time.Second)
		if execErr != nil {
			log.Printf("Worker %d ERROR executing job %s (Type: %s): %v", id, job.ID, execErr)
			// Mark failed
			if markErr := queue.MarkFailed(updateCtx, job.ID, execErr); markErr != nil {
				log.Printf("Worker %d ERROR: Failed to mark job %s as failed after execution error: %v", id, job.ID, markErr)
			}
		} else {
			log.Printf("Worker %d finished job %s (Type: %s) successfully", id, job.ID)
			// Mark done
			if markErr := queue.MarkDone(updateCtx, job.ID); markErr != nil {
				log.Printf("Worker %d ERROR: Failed to mark job %s as done: %v", id, job.ID, markErr)
			}
		}
		cancelUpdate()
	}
}

// --- Enqueue Function ---
// Type hint changes to *EntQueue
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
