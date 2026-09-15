package discovery

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type staticAdapter struct {
	name   string
	result Result
}

func (adapter staticAdapter) Name() string {
	return adapter.name
}

func (adapter staticAdapter) Discover(context.Context, Options) Result {
	return adapter.result
}

func TestDiscoverWithAdaptersOrdersAndDeduplicates(t *testing.T) {
	duplicate := inventory.Artifact{
		ID:     "skill:review",
		Name:   "review",
		Kind:   "agent-skill",
		Path:   "/repo/.github/skills/review/SKILL.md",
		Source: "/repo/.github/skills",
		Scope:  "project",
	}
	found, err := DiscoverWithAdapters(context.Background(), Options{}, []Adapter{
		staticAdapter{name: "second", result: Result{
			Artifacts: []inventory.Artifact{
				{ID: "cursor:z", Name: "z", Kind: "ide-extension", Source: "cursor", Scope: "personal"},
				duplicate,
			},
			Diagnostics: []inventory.Diagnostic{
				{Level: "info", Provider: "second", Code: "provider_unavailable", Message: "unavailable"},
				{Level: "info", Provider: "second", Code: "provider_unavailable", Message: "unavailable"},
			},
		}},
		staticAdapter{name: "first", result: Result{
			Artifacts: []inventory.Artifact{
				duplicate,
				{ID: "skill:a", Name: "a", Kind: "agent-skill", Path: "/repo/a/SKILL.md", Source: "/repo/a", Scope: "project"},
			},
			Diagnostics: []inventory.Diagnostic{
				{Level: "error", Provider: "first", Code: "location_unreadable", Message: "unreadable"},
				{Level: "warning", Provider: "first", Code: "malformed_artifact", Message: "malformed"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(found.Artifacts) != 3 {
		t.Fatalf("got %d artifacts, want 3", len(found.Artifacts))
	}
	if found.Artifacts[0].ID != "skill:a" || found.Artifacts[1].ID != "skill:review" {
		t.Fatalf("artifacts are not ordered: %#v", found.Artifacts)
	}
	levels := []string{
		found.Diagnostics[0].Level,
		found.Diagnostics[1].Level,
		found.Diagnostics[2].Level,
	}
	if !reflect.DeepEqual(levels, []string{"error", "warning", "info"}) {
		t.Fatalf("diagnostics are not ordered: %#v", found.Diagnostics)
	}
}

func TestDiscoverWithAdaptersAddsProviderToDiagnostic(t *testing.T) {
	found, err := DiscoverWithAdapters(context.Background(), Options{}, []Adapter{
		staticAdapter{name: "provider", result: Result{
			Diagnostics: []inventory.Diagnostic{{
				Level:   "warning",
				Code:    "partial",
				Message: "partial result",
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if found.Diagnostics[0].Provider != "provider" {
		t.Fatalf("got provider %q", found.Diagnostics[0].Provider)
	}
}

func TestDiscoverWithAdaptersKeepsDistinctArtifactsSharingPath(t *testing.T) {
	path := "/repo/.mcp.json"
	found, err := DiscoverWithAdapters(context.Background(), Options{}, []Adapter{
		staticAdapter{name: "mcp", result: Result{
			Artifacts: []inventory.Artifact{
				{
					ID:     "mcp:portable-mcp:project:local",
					Name:   "local",
					Kind:   "mcp-server",
					Path:   path,
					Source: "portable-mcp",
					Scope:  "project",
				},
				{
					ID:     "mcp:portable-mcp:project:remote",
					Name:   "remote",
					Kind:   "mcp-server",
					Path:   path,
					Source: "portable-mcp",
					Scope:  "project",
				},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2: %#v", len(found.Artifacts), found.Artifacts)
	}
}

func TestSkillAdapterDiscoversManifest(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".github", "skills", "safe-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "---\nname: safe-review\ndescription: Reviews changes\nallowed-tools: read\n---\n# Safe Review\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}

	result := newSkillAdapter().Discover(context.Background(), Options{
		Root: root,
		Home: t.TempDir(),
	})
	if len(result.Artifacts) != 1 {
		t.Fatalf("got %d artifacts, want 1", len(result.Artifacts))
	}
	if result.Artifacts[0].Name != "safe-review" {
		t.Fatalf("got name %q", result.Artifacts[0].Name)
	}
	if result.Artifacts[0].Metadata["description"] != "Reviews changes" {
		t.Fatalf("got metadata %#v", result.Artifacts[0].Metadata)
	}
	if len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
	}
}

func TestSkillAdapterReportsMalformedManifest(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".github", "skills", "broken")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := newSkillAdapter().Discover(context.Background(), Options{
		Root: root,
		Home: t.TempDir(),
	})
	if len(result.Artifacts) != 1 || result.Artifacts[0].Name != "broken" {
		t.Fatalf("malformed skill was not inventoried: %#v", result.Artifacts)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "malformed_artifact" {
		t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Path != ".github/skills/broken/SKILL.md" {
		t.Fatalf("got diagnostic path %q", result.Diagnostics[0].Path)
	}
}

func TestSkillAdapterReportsConfiguredMissingLocationWithoutExposingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "private", "skills")
	t.Setenv("COPILOT_SKILLS_DIRS", missing)

	result := newSkillAdapter().Discover(context.Background(), Options{
		Root: t.TempDir(),
		Home: t.TempDir(),
	})
	if len(result.Diagnostics) != 1 {
		t.Fatalf("got diagnostics %#v", result.Diagnostics)
	}
	diagnostic := result.Diagnostics[0]
	if diagnostic.Code != "configured_location_unavailable" || diagnostic.Path != "skills" {
		t.Fatalf("unexpected diagnostic %#v", diagnostic)
	}
}

type failingFileSystem struct {
	osFileSystem
	readDirError error
}

func (files failingFileSystem) ReadDir(string) ([]os.DirEntry, error) {
	return nil, files.readDirError
}

func TestSkillAdapterReportsPermissionFailure(t *testing.T) {
	adapter := skillAdapter{files: failingFileSystem{readDirError: os.ErrPermission}}
	result := adapter.discoverRoot(
		skillRoot{path: "/repo/.github/skills", scope: "project"},
		Options{Root: "/repo", Home: "/home/user"},
	)
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "location_unreadable" {
		t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Level != "error" {
		t.Fatalf("got level %q", result.Diagnostics[0].Level)
	}
}

func TestEditorAdapterDiscoversExtensions(t *testing.T) {
	runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte("github.copilot@1.2.3\nms-python.python@2026.1.0\n"), nil
	}

	result := newEditorAdapter("visual-studio-code", "code", runner).
		Discover(context.Background(), Options{})
	if len(result.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(result.Artifacts))
	}
	if result.Artifacts[0].Version != "1.2.3" {
		t.Fatalf("got version %q", result.Artifacts[0].Version)
	}
}

func TestEditorAdapterReportsUnavailableAndMalformedOutput(t *testing.T) {
	t.Run("unavailable", func(t *testing.T) {
		runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, &exec.Error{Name: "code", Err: exec.ErrNotFound}
		}
		result := newEditorAdapter("visual-studio-code", "code", runner).
			Discover(context.Background(), Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "provider_unavailable" {
			t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
		}
		if result.Diagnostics[0].Level != "info" {
			t.Fatalf("got level %q", result.Diagnostics[0].Level)
		}
	})

	t.Run("failed", func(t *testing.T) {
		expected := errors.New("editor failed")
		runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, expected
		}
		result := newEditorAdapter("cursor", "cursor", runner).
			Discover(context.Background(), Options{})
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "provider_failed" {
			t.Fatalf("unexpected diagnostics %#v", result.Diagnostics)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		runner := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte("missing-version\nvalid.extension@1.0.0\n"), nil
		}
		result := newEditorAdapter("cursor", "cursor", runner).
			Discover(context.Background(), Options{})
		if len(result.Artifacts) != 1 || len(result.Diagnostics) != 1 {
			t.Fatalf("unexpected result %#v", result)
		}
		if result.Diagnostics[0].Code != "malformed_output" {
			t.Fatalf("unexpected diagnostic %#v", result.Diagnostics[0])
		}
	})
}
