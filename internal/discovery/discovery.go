package discovery

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type Options struct {
	Root string
	Home string
}

type Result struct {
	Artifacts   []inventory.Artifact
	Diagnostics []inventory.Diagnostic
}

type Adapter interface {
	Name() string
	Discover(context.Context, Options) Result
}

func Discover(ctx context.Context, options Options) (inventory.Inventory, error) {
	return DiscoverWithAdapters(ctx, options, defaultAdapters())
}

func DiscoverWithAdapters(ctx context.Context, options Options, adapters []Adapter) (inventory.Inventory, error) {
	resolved, err := resolveOptions(options)
	if err != nil {
		return inventory.Inventory{}, err
	}
	found := inventory.Inventory{
		Artifacts:   make([]inventory.Artifact, 0),
		Diagnostics: make([]inventory.Diagnostic, 0),
	}
	seen := make(map[string]struct{})
	seenDiagnostics := make(map[string]struct{})

	for _, adapter := range adapters {
		result := adapter.Discover(ctx, resolved)
		for _, artifact := range result.Artifacts {
			key := artifactKey(artifact)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			found.Artifacts = append(found.Artifacts, artifact)
		}
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Provider == "" {
				diagnostic.Provider = adapter.Name()
			}
			key := diagnosticKey(diagnostic)
			if _, exists := seenDiagnostics[key]; exists {
				continue
			}
			seenDiagnostics[key] = struct{}{}
			found.Diagnostics = append(found.Diagnostics, diagnostic)
		}
	}

	sort.Slice(found.Artifacts, func(i, j int) bool {
		left := found.Artifacts[i]
		right := found.Artifacts[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.ID != right.ID {
			return left.ID < right.ID
		}
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		return left.Source < right.Source
	})
	sort.Slice(found.Diagnostics, func(i, j int) bool {
		left := found.Diagnostics[i]
		right := found.Diagnostics[j]
		if diagnosticRank(left.Level) != diagnosticRank(right.Level) {
			return diagnosticRank(left.Level) < diagnosticRank(right.Level)
		}
		if left.Provider != right.Provider {
			return left.Provider < right.Provider
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Message < right.Message
	})
	return found, nil
}

func resolveOptions(options Options) (Options, error) {
	if options.Root == "" {
		options.Root = "."
	}
	if options.Home == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Options{}, fmt.Errorf("resolve home directory: %w", err)
		}
		options.Home = home
	}
	return options, nil
}

func defaultAdapters() []Adapter {
	runner := execCommand
	return []Adapter{
		newSkillAdapter(),
		newEditorAdapter("visual-studio-code", "code", runner),
		newEditorAdapter("cursor", "cursor", runner),
	}
}

func artifactKey(artifact inventory.Artifact) string {
	if artifact.Path != "" {
		return artifact.Kind + "\x00" + artifact.Path
	}
	return artifact.Kind + "\x00" + artifact.ID + "\x00" + artifact.Scope + "\x00" + artifact.Source
}

func diagnosticKey(diagnostic inventory.Diagnostic) string {
	return diagnostic.Level + "\x00" +
		diagnostic.Provider + "\x00" +
		diagnostic.Code + "\x00" +
		diagnostic.Path + "\x00" +
		diagnostic.Message
}

func diagnosticRank(level string) int {
	switch level {
	case "error":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}
