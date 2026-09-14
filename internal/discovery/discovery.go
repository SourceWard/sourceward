package discovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type Options struct {
	Root string
	Home string
}

func Discover(ctx context.Context, options Options) (inventory.Inventory, error) {
	if options.Root == "" {
		options.Root = "."
	}
	if options.Home == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return inventory.Inventory{}, fmt.Errorf("resolve home directory: %w", err)
		}
		options.Home = home
	}

	skills, err := discoverSkills(options)
	if err != nil {
		return inventory.Inventory{}, err
	}

	artifacts := append([]inventory.Artifact{}, skills...)
	artifacts = append(artifacts, discoverExtensions(ctx, execCommand)...)
	sort.Slice(artifacts, func(i, j int) bool {
		if artifacts[i].Kind == artifacts[j].Kind {
			return artifacts[i].ID < artifacts[j].ID
		}
		return artifacts[i].Kind < artifacts[j].Kind
	})

	return inventory.Inventory{Artifacts: artifacts}, nil
}

func discoverSkills(options Options) ([]inventory.Artifact, error) {
	projectRoots := []string{
		filepath.Join(options.Root, ".github", "skills"),
		filepath.Join(options.Root, ".agents", "skills"),
		filepath.Join(options.Root, ".claude", "skills"),
	}
	personalRoots := []string{
		filepath.Join(options.Home, ".copilot", "skills"),
		filepath.Join(options.Home, ".agents", "skills"),
	}

	if configured := os.Getenv("COPILOT_SKILLS_DIRS"); configured != "" {
		for _, root := range strings.Split(configured, ",") {
			if root = strings.TrimSpace(root); root != "" {
				personalRoots = append(personalRoots, root)
			}
		}
	}

	var artifacts []inventory.Artifact
	seen := make(map[string]struct{})
	for _, candidate := range []struct {
		roots []string
		scope string
	}{
		{roots: projectRoots, scope: "project"},
		{roots: personalRoots, scope: "personal"},
	} {
		for _, root := range candidate.roots {
			found, err := skillsInRoot(root, candidate.scope)
			if err != nil {
				return nil, err
			}
			for _, artifact := range found {
				key := artifact.Kind + "\x00" + artifact.Path
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				artifacts = append(artifacts, artifact)
			}
		}
	}
	return artifacts, nil
}

func skillsInRoot(root, scope string) ([]inventory.Artifact, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skill directory %s: %w", root, err)
	}

	var artifacts []inventory.Artifact
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillFile := filepath.Join(root, entry.Name(), "SKILL.md")
		info, err := os.Stat(skillFile)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect skill %s: %w", skillFile, err)
		}
		if info.IsDir() {
			continue
		}

		name, metadata, err := parseSkillHeader(skillFile)
		if err != nil {
			return nil, err
		}
		if name == "" {
			name = entry.Name()
		}
		artifacts = append(artifacts, inventory.Artifact{
			ID:       "skill:" + name,
			Name:     name,
			Kind:     "agent-skill",
			Path:     skillFile,
			Source:   root,
			Scope:    scope,
			Metadata: metadata,
		})
	}
	return artifacts, nil
}

func parseSkillHeader(path string) (string, map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("read skill manifest %s: %w", path, err)
	}
	text := string(content)
	if !strings.HasPrefix(text, "---\n") {
		return "", nil, nil
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", nil, nil
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
	return metadata["name"], metadata, nil
}

func discoverExtensions(ctx context.Context, runner commandRunner) []inventory.Artifact {
	editors := []struct {
		command string
		source  string
	}{
		{command: "code", source: "visual-studio-code"},
		{command: "cursor", source: "cursor"},
	}

	var artifacts []inventory.Artifact
	for _, editor := range editors {
		output, err := runner(ctx, editor.command, "--list-extensions", "--show-versions")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(output), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			name, version, _ := strings.Cut(line, "@")
			artifacts = append(artifacts, inventory.Artifact{
				ID:      editor.source + ":" + name,
				Name:    name,
				Kind:    "ide-extension",
				Version: version,
				Source:  editor.source,
				Scope:   "personal",
			})
		}
	}
	return artifacts
}

func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}
