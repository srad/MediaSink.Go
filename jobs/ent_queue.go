package jobs

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/srad/mediasink/ent"
	"github.com/srad/mediasink/ent/job"
	"log"
	"sync"
	"time"
)

// EntQueue implements a persistent job queue using SQLite via Ent ORM.
type EntQueue struct {
	client     *ent.Client // Use Ent Client
	mu         sync.Mutex
	notifyChan chan struct{}
	closeOnce  sync.Once
	closed     bool
}

func NewEntQueue(client *ent.Client) (*EntQueue, error) {
	q := &EntQueue{
		client:     client,
		notifyChan: make(chan struct{}, 1), // Buffered channel
	}

	return q, nil
}

// Helper to map ent.Job to internal Job struct
func mapEntJobToJob(entJob *ent.Job) Job {
	return Job{
		ID:      entJob.ID.String(), // Convert UUID to string for internal Job struct
		Type:    entJob.Type,
		Payload: entJob.Payload,
		// Include other fields if your internal Job struct needs them (status, attempts etc.)
	}
}

// Enqueue adds a job to the database using the Ent client.
func (q *EntQueue) Enqueue(job Job) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return fmt.Errorf("queue closed")
	}
	q.mu.Unlock()

	jobUUID, err := uuid.Parse(job.ID) // Parse string ID back to UUID
	if err != nil {
		// Handle error or generate new UUID if ID wasn't pre-set
		log.Printf("Warning: Could not parse job ID %s as UUID, generating new one: %v", job.ID, err)
		jobUUID = uuid.New()
	}

	_, err = q.client.Job.
		Create().
		SetID(jobUUID). // Use the UUID
		SetType(job.Type).
		SetPayload(job.Payload).
		SetStatus("pending"). // Use generated enum value
		// CreatedAt, UpdatedAt, AttemptCount defaults handled by schema/Ent
		Save(context.Background()) // Use appropriate context

	if err != nil {
		return fmt.Errorf("ent failed to enqueue job %s: %w", job.ID, err)
	}

	// Notify a waiting worker
	select {
	case q.notifyChan <- struct{}{}:
	default:
	}

	log.Printf("Enqueued job %s (Type: %s) into SQLite via Ent", job.ID, job.Type)
	return nil
}

// Dequeue retrieves and locks the oldest pending job using Ent transactions.
func (q *EntQueue) Dequeue(ctx context.Context) (Job, error) {
	for {
		q.mu.Lock()
		if q.closed {
			q.mu.Unlock()
			// Use a specific error or check ctx.Err() outside
			return Job{}, fmt.Errorf("queue closed")
		}
		q.mu.Unlock()

		var dequeuedJob Job
		var jobFound bool

		// Use Ent's transaction helper TxFromContext or manual Tx
		tx, err := q.client.Tx(ctx)
		if err != nil {
			log.Printf("ERROR: Failed to begin Ent transaction: %v", err)
			// Back off slightly
			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return Job{}, ctx.Err()
			}
		}

		// --- Execute Dequeue Logic within Transaction ---
		jobToRun, err := tx.Job.
			Query().
			Where(job.StatusEQ(job.StatusPending)). // Use generated predicate and enum value
			Order(ent.Asc(job.FieldCreatedAt)).     // Use generated field constant
			First(ctx)                              // Use the transaction's context

		if err != nil {
			// Rollback is automatic if error is returned from tx function or explicitly called
			if ent.IsNotFound(err) {
				_ = tx.Rollback() // Explicit rollback just in case, though likely automatic
				// No job found, wait outside the transaction
			} else {
				_ = tx.Rollback()
				log.Printf("ERROR: Ent transaction failed during dequeue find: %v", err)
				// Back off on other errors before retrying
				select {
				case <-time.After(1 * time.Second):
					// Continue loop to retry transaction
				case <-ctx.Done():
					return Job{}, ctx.Err()
				}
				continue // Retry the transaction after backoff
			}
			// If NotFound, proceed to wait logic outside Tx block
		} else {
			// Job found, attempt to update it within the same transaction
			updatedJob, updateErr := jobToRun.Update().
				SetStatus(job.StatusRunning). // Use generated enum
				SetAttemptCount(jobToRun.AttemptCount + 1).
				// UpdatedAt handled by Ent schema hook/default
				Save(ctx) // Use transaction's context

			if updateErr != nil {
				_ = tx.Rollback()
				log.Printf("ERROR: Ent transaction failed during dequeue update for job %s: %v", jobToRun.ID, updateErr)
				// Back off slightly before retrying the whole dequeue
				select {
				case <-time.After(500 * time.Millisecond):
					// Continue outer loop
				case <-ctx.Done():
					return Job{}, ctx.Err()
				}
				continue // Retry the transaction
			}

			// Update successful, commit the transaction
			commitErr := tx.Commit()
			if commitErr != nil {
				log.Printf("ERROR: Ent transaction commit failed after dequeue update for job %s: %v", updatedJob.ID, commitErr)
				// Treat as failure and retry dequeue
				select {
				case <-time.After(500 * time.Millisecond):
					// Continue outer loop
				case <-ctx.Done():
					return Job{}, ctx.Err()
				}
				continue
			}

			// Transaction successful
			jobFound = true
			dequeuedJob = mapEntJobToJob(updatedJob)
		}
		// --- End Transaction Logic ---

		// After transaction attempt:
		if jobFound {
			log.Printf("Dequeued job %s (Type: %s) from SQLite via Ent", dequeuedJob.ID, dequeuedJob.Type)
			return dequeuedJob, nil // Successfully dequeued and locked
		}

		// No job was found or an error occurred and was handled, wait before next attempt
		waitTimeout := time.NewTimer(2 * time.Second)
		select {
		case <-q.notifyChan:
			waitTimeout.Stop()
			continue // Notification received, try again immediately
		case <-ctx.Done():
			waitTimeout.Stop()
			return Job{}, ctx.Err() // Context cancelled (shutdown)
		case <-waitTimeout.C:
			continue // Timed out, loop again to poll DB
		}
	}
}

// MarkDone updates the job status to 'done' using the Ent client.
func (q *EntQueue) MarkDone(ctx context.Context, jobID string) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return fmt.Errorf("queue closed")
	}
	q.mu.Unlock()

	jobUUID, err := uuid.Parse(jobID)
	if err != nil {
		return fmt.Errorf("invalid job ID format for MarkDone: %w", err)
	}

	// Update using UpdateOneID for specific job
	_, err = q.client.Job.
		UpdateOneID(jobUUID).                   // Find by UUID
		Where(job.StatusEQ(job.StatusRunning)). // Ensure current status
		SetStatus(job.StatusDone).              // Set new status using generated enum
		// UpdatedAt handled by Ent
		Save(ctx) // Use provided context

	if err != nil {
		if ent.IsNotFound(err) {
			// Job didn't exist or wasn't in 'running' state
			log.Printf("Warning: Job %s not marked done via Ent (not found or status was not 'running')", jobID)
			return nil // Or return a specific error if needed
		}
		return fmt.Errorf("ent failed to mark job %s as done: %w", jobID, err)
	}
	return nil
}

// MarkFailed updates the job status to 'failed' using the Ent client.
func (q *EntQueue) MarkFailed(ctx context.Context, jobID string, jobErr error) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return fmt.Errorf("queue closed")
	}
	q.mu.Unlock()

	jobUUID, err := uuid.Parse(jobID)
	if err != nil {
		return fmt.Errorf("invalid job ID format for MarkFailed: %w", err)
	}

	errMsg := "Unknown error"
	if jobErr != nil {
		errMsg = jobErr.Error()
	}

	job, err := q.client.Job.
		UpdateOneID(jobUUID).
		Where(job.StatusEQ(job.StatusRunning)).
		SetStatus(job.StatusFailed). // Set status to failed
		SetLastError(errMsg).        // Set the error message
		// UpdatedAt handled by Ent
		Save(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			log.Printf("Warning: Job %s not marked failed via Ent (not found or status was not 'running')", jobID)
			return nil // Or return a specific error
		}
		return fmt.Errorf("ent failed to mark job %s as failed: %w", jobID, err)
	}
	if job == nil {
		log.Printf("Warning: Job %s update query affected 0 rows unexpectedly during MarkFailed.", jobID)
	} else {
		log.Printf("Marked job %s as failed via Ent. Error: %s", jobID, errMsg)
	}
	return nil
}

// Close closes the Ent client connection.
func (q *EntQueue) Close() error {
	var err error
	q.closeOnce.Do(func() {
		q.mu.Lock()
		q.closed = true
		q.mu.Unlock()
		close(q.notifyChan)
		log.Println("Closing Ent client connection...")
		err = q.client.Close()
		if err != nil {
			log.Printf("Error closing Ent client: %v", err)
		} else {
			log.Println("Ent client connection closed.")
		}
	})
	return err
}
