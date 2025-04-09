package jobs

import "context"

// JobPayload defines the data structure for job data.
// Using json.RawMessage allows flexibility; plugins unmarshal it as needed.
type JobPayload []byte // Or map[string]interface{}, but []byte is often better

// ProgressInfo holds information about the job's execution progress.
type ProgressInfo struct {
	// Optional: You might want JobID here if progress is handled centrally
	// JobID        string      `json:"jobId,omitempty"`

	// Percentage complete (e.g., 0.0 to 100.0)
	Percentage float64 `json:"percentage"`

	// Description of the current step or status
	CurrentStep string `json:"currentStep"`

	// Optional field for any other structured details
	Details *string `json:"details,omitempty"`
}

// JobWorker defines the interface that all job plugins must implement.
type JobWorker interface {
	// Type returns the unique identifier for the job type this plugin handles.
	// This string will be used to match jobs to plugins.
	Type() string

	// Execute performs the job's task.
	// Takes context for cancellation and the job's payload.
	Execute(ctx context.Context, payload JobPayload, progressChan chan<- ProgressInfo) error
}

// PluginSymbol is the expected name of the exported symbol in the plugin .so file
// that provides the JobWorker implementation. It should be a variable or function
// returning a JobWorker. For simplicity, we'll use a variable.
const PluginSymbol = "Worker" // Or a function like "NewWorker" returning JobWorker
