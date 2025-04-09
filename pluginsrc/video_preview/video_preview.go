package main

import (
	"context"
	"fmt"
	"github.com/srad/mediasink/jobs"
	"time"
)

// Important: This plugin MUST be built with the exact same Go version
// and dependency versions (especially the 'jobspec' package) as the main application.

import (
	"encoding/json"
	"log"
)

type VideoPreviewsPayload struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
}

// sendEmailWorker implements the jobspec.JobWorker interface
type videoPreviewsWorker struct{}

// Type returns the identifier for this job type
func (w *videoPreviewsWorker) Type() string {
	return "video_previews"
}

// Execute performs the email sending task
func (w *videoPreviewsWorker) Execute(ctx context.Context, payload jobs.JobPayload) error {
	var data VideoPreviewsPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("failed to unmarshal video_previews payload: %w", err)
	}

	// --- Simulate work & context cancellation ---
	select {
	case <-time.After(2 * time.Second): // Simulate sending delay
		// ** Actual email sending logic would go here **
		log.Printf("PLUGIN [video_previews]: Successfully converted file to %s", data.OutputPath)
		return nil // Indicate success
	case <-ctx.Done():
		log.Printf("PLUGIN [video_previews]: Execution cancelled for %s", data.InputPath)
		return ctx.Err() // Propagate cancellation error
	}
}

// Export the worker instance using the agreed symbol name.
// IMPORTANT: This variable name ("Worker") MUST match jobspec.PluginSymbol
var Worker jobs.JobWorker = &videoPreviewsWorker{}

// You need a main function, but it's not used when loaded as a plugin.
func main() {}
