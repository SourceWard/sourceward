package discovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type fileSystem interface {
	ReadDir(string) ([]os.DirEntry, error)
	Stat(string) (os.FileInfo, error)
	ReadFile(string) ([]byte, error)
}

type osFileSystem struct{}

func (osFileSystem) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (osFileSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (osFileSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type skillAdapter struct {
	files fileSystem
}

type skillRoot struct {
	path       string
	scope      string
	configured bool
}

func newSkillAdapter() Adapter {
	return skillAdapter{files: osFileSystem{}}
}

func (skillAdapter) Name() string {
	return "agent-skills"
}

func (adapter skillAdapter) Discover(_ context.Context, options Options) Result {
	roots := []skillRoot{
		{path: filepath.Join(options.Root, ".github", "skills"), scope: "project"},
		{path: filepath.Join(options.Root, ".agents", "skills"), scope: "project"},
		{path: filepath.Join(options.Root, ".claude", "skills"), scope: "project"},
		{path: filepath.Join(options.Home, ".copilot", "skills"), scope: "personal"},
		{path: filepath.Join(options.Home, ".agents", "skills"), scope: "personal"},
	}
	if configured := os.Getenv("COPILOT_SKILLS_DIRS"); configured != "" {
		for _, root := range strings.Split(configured, ",") {
			if root = strings.TrimSpace(root); root != "" {
				roots = append(roots, skillRoot{path: root, scope: "personal", configured: true})
			}
		}
	}

	result := Result{}
	for _, root := range roots {
		found := adapter.discoverRoot(root, options)
		result.Artifacts = append(result.Artifacts, found.Artifacts...)
		result.Diagnostics = append(result.Diagnostics, found.Diagnostics...)
	}
	return result
}

func (adapter skillAdapter) discoverRoot(root skillRoot, options Options) Result {
	entries, err := adapter.files.ReadDir(root.path)
	if errors.Is(err, os.ErrNotExist) {
		if !root.configured {
			return Result{}
		}
		return Result{Diagnostics: []inventory.Diagnostic{{
			Code:     "configured_location_unavailable",
			Level:    "warning",
			Provider: adapter.Name(),
			Message:  "configured skill directory does not exist",
			Path:     displayPath(root.path, options),
		}}}
	}
	if err != nil {
		return Result{Diagnostics: []inventory.Diagnostic{{
			Code:     "location_unreadable",
			Level:    "error",
			Provider: adapter.Name(),
			Message:  "skill directory could not be read",
			Path:     displayPath(root.path, options),
		}}}
	}

	result := Result{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(root.path, entry.Name(), "SKILL.md")
		info, err := adapter.files.Stat(skillFile)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
				Code:     "artifact_unreadable",
				Level:    "error",
				Provider: adapter.Name(),
				Message:  "skill manifest could not be inspected",
				Path:     displayPath(skillFile, options),
			})
			continue
		}
		if info.IsDir() {
			result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
				Code:     "malformed_artifact",
				Level:    "warning",
				Provider: adapter.Name(),
				Message:  "SKILL.md is a directory rather than a file",
				Path:     displayPath(skillFile, options),
			})
			continue
		}

		content, err := adapter.files.ReadFile(skillFile)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
				Code:     "artifact_unreadable",
				Level:    "error",
				Provider: adapter.Name(),
				Message:  "skill manifest could not be read",
				Path:     displayPath(skillFile, options),
			})
			continue
		}
		name, metadata, malformed := parseSkillHeader(content)
		if name == "" {
			name = entry.Name()
		}
		result.Artifacts = append(result.Artifacts, inventory.Artifact{
			ID:       "skill:" + name,
			Name:     name,
			Kind:     "agent-skill",
			Path:     skillFile,
			Source:   root.path,
			Scope:    root.scope,
			Metadata: metadata,
		})
		if malformed {
			result.Diagnostics = append(result.Diagnostics, inventory.Diagnostic{
				Code:     "malformed_artifact",
				Level:    "warning",
				Provider: adapter.Name(),
				Message:  "skill frontmatter is not terminated",
				Path:     displayPath(skillFile, options),
			})
		}
	}
	return result
}

func parseSkillHeader(content []byte) (string, map[string]string, bool) {
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return "", nil, false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", nil, true
	}

	metadata := make(map[string]string)
	for _, line := range strings.Split(text[4:4+end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "name" || key == "description" || key == "allowed-tools" {
			metadata[key] = value
		}
	}
	return metadata["name"], metadata, false
}

func displayPath(path string, options Options) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if relative, ok := pathWithin(options.Root, absolute); ok {
		if relative == "." {
			return "."
		}
		return filepath.ToSlash(relative)
	}
	if relative, ok := pathWithin(options.Home, absolute); ok {
		if relative == "." {
			return "~"
		}
		return "~/" + filepath.ToSlash(relative)
	}
	return filepath.Base(filepath.Clean(path))
}

func pathWithin(root, path string) (string, bool) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(absoluteRoot, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return relative, true
}
