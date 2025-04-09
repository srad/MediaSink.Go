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

type StripePreviewPayload struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
}

// sendEmailWorker implements the jobspec.JobWorker interface
type stripePreviewWorker struct{}

// Type returns the identifier for this job type
func (w *stripePreviewWorker) Type() string {
	return "stripe_preview"
}

// Execute performs the email sending task
func (w *stripePreviewWorker) Execute(ctx context.Context, payload jobs.JobPayload) error {
	var data StripePreviewPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("failed to unmarshal stripe_preview payload: %w", err)
	}

	// --- Simulate work & context cancellation ---
	select {
	case <-time.After(2 * time.Second): // Simulate sending delay
		// ** Actual email sending logic would go here **
		log.Printf("PLUGIN [stripe_preview]: Successfully converted file to %s", data.OutputPath)
		return nil // Indicate success
	case <-ctx.Done():
		log.Printf("PLUGIN [stripe_preview]: Execution cancelled for %s", data.InputPath)
		return ctx.Err() // Propagate cancellation error
	}
}

// Export the worker instance using the agreed symbol name.
// IMPORTANT: This variable name ("Worker") MUST match jobspec.PluginSymbol
var Worker jobs.JobWorker = &stripePreviewWorker{}

// You need a main function, but it's not used when loaded as a plugin.
func main() {}
