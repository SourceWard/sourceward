package discovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsInRoot(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "safe-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "---\nname: safe-review\ndescription: Reviews changes\nallowed-tools: read\n---\n# Safe Review\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	artifacts, err := skillsInRoot(root, "project")
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("got %d artifacts, want 1", len(artifacts))
	}
	if artifacts[0].Name != "safe-review" {
		t.Fatalf("got name %q", artifacts[0].Name)
	}
	if artifacts[0].Metadata["description"] != "Reviews changes" {
		t.Fatalf("got metadata %#v", artifacts[0].Metadata)
	}
}

func TestDiscoverExtensions(t *testing.T) {
	runner := func(_ context.Context, name string, _ ...string) ([]byte, error) {
		if name == "code" {
			return []byte("github.copilot@1.2.3\nms-python.python@2026.1.0\n"), nil
		}
		return nil, os.ErrNotExist
	}

	artifacts := discoverExtensions(context.Background(), runner)
	if len(artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(artifacts))
	}
	if artifacts[0].Version != "1.2.3" {
		t.Fatalf("got version %q", artifacts[0].Version)
	}
}
