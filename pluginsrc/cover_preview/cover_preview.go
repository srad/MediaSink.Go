package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/srad/mediasink/helpers"
	"github.com/srad/mediasink/jobs"
)

// Important: This plugin MUST be built with the exact same Go version
// and dependency versions (especially the 'jobs' package) as the main application.

type CoverPreviewPayload struct {
	InputPath  string `json:"inputPath"`
	OutputPath string `json:"outputPath"`
	Filename   string `json:"filename"`
}

// sendEmailWorker implements the jobspec.JobWorker interface
type coverPreviewWorker struct{}

// Type returns the identifier for this job type
func (w *coverPreviewWorker) Type() string {
	return "cover_preview"
}

// Execute performs the email sending task
func (w *coverPreviewWorker) Execute(ctx context.Context, payload jobs.JobPayload) error {
	var data CoverPreviewPayload
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("failed to unmarshal cover_preview payload: %w", err)
	}

	_, err := helpers.Video{FilePath: data.InputPath}.ExecPreviewCover(data.OutputPath, data.Filename, func(info helpers.CommandInfo) {
		job.UpdateInfo(info.Pid, info.Command)
	}, func(message helpers.PipeMessage) {
		job.Error(errors.New(message.Output))
	})
	if err != nil {
		return fmt.Errorf("failed to execute cover_preview job: %v", err)
	}

	return nil
}

// Export the worker instance using the agreed symbol name.
// IMPORTANT: This variable name ("Worker") MUST match jobspec.PluginSymbol
var Worker jobs.JobWorker = &coverPreviewWorker{}

// You need a main function, but it's not used when loaded as a plugin.
func main() {}
