package discovery

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type editorAdapter struct {
	name    string
	command string
	runner  commandRunner
}

func newEditorAdapter(name, command string, runner commandRunner) Adapter {
	return editorAdapter{name: name, command: command, runner: runner}
}

func (adapter editorAdapter) Name() string {
	return adapter.name
}

func (adapter editorAdapter) Discover(ctx context.Context, _ Options) Result {
	output, err := adapter.runner(ctx, adapter.command, "--list-extensions", "--show-versions")
	if err != nil {
		level := "warning"
		code := "provider_failed"
		message := "editor extension inventory command failed"
		if isCommandUnavailable(err) {
			level = "info"
			code = "provider_unavailable"
			message = "editor command is not installed or not available on PATH"
		}
		return Result{Diagnostics: []inventory.Diagnostic{{
			Code:     code,
			Level:    level,
			Provider: adapter.Name(),
			Message:  message,
		}}}
	}

	result := Result{}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, version, ok := strings.Cut(line, "@")
		if !ok || name == "" || version == "" {
			result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
				Code:     "malformed_output",
				Level:    "warning",
				Provider: adapter.Name(),
				Message:  "editor returned an extension without a name and version",
			})
			continue
		}
		result.Artifacts = append(result.Artifacts, inventory.Artifact{
			ID:      adapter.name + ":" + name,
			Name:    name,
			Kind:    "ide-extension",
			Version: version,
			Source:  adapter.name,
			Scope:   "personal",
		})
	}
	return result
}

func isCommandUnavailable(err error) bool {
	var execError *exec.Error
	return errors.As(err, &execError) && errors.Is(execError.Err, exec.ErrNotFound)
}

func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
