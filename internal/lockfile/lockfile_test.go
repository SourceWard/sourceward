package lockfile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/SourceWard/sourceward/internal/inventory"
)

func TestGenerateHashesSkillContentsDeterministically(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, ".github", "skills", "review")
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("# Review\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "check.sh"), []byte("exit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	found := inventory.Inventory{Artifacts: []inventory.Artifact{{
		ID:     "skill:review",
		Name:   "review",
		Kind:   "agent-skill",
		Path:   skillPath,
		Source: filepath.Dir(skillDir),
		Scope:  "project",
	}}}

	first, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("lockfiles differ:\n%#v\n%#v", first, second)
	}
	if first.Artifacts[0].Source != ".github/skills" {
		t.Fatalf("got source %q", first.Artifacts[0].Source)
	}
	if first.Artifacts[0].Integrity.Scope != "content" {
		t.Fatalf("got integrity scope %q", first.Artifacts[0].Integrity.Scope)
	}

	originalDigest := first.Artifacts[0].Integrity.Digest
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "check.sh"), []byte("exit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	changed, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if changed.Artifacts[0].Integrity.Digest == originalDigest {
		t.Fatal("digest did not change with skill contents")
	}
}

func TestGenerateFiltersPersonalArtifactsAndSorts(t *testing.T) {
	found := inventory.Inventory{Artifacts: []inventory.Artifact{
		{ID: "z", Name: "z", Kind: "ide-extension", Version: "1", Source: "cursor", Scope: "personal"},
		{ID: "b", Name: "b", Kind: "ide-extension", Version: "1", Source: "test", Scope: "project"},
		{ID: "a", Name: "a", Kind: "ide-extension", Version: "1", Source: "test", Scope: "project"},
	}}

	locked, err := Generate(found, Options{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if len(locked.Artifacts) != 2 {
		t.Fatalf("got %d artifacts, want 2", len(locked.Artifacts))
	}
	if locked.Artifacts[0].ID != "a" || locked.Artifacts[1].ID != "b" {
		t.Fatalf("artifacts are not sorted: %#v", locked.Artifacts)
	}

	withPersonal, err := Generate(found, Options{Root: t.TempDir(), IncludePersonal: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(withPersonal.Artifacts) != 3 {
		t.Fatalf("got %d artifacts, want 3", len(withPersonal.Artifacts))
	}
	if withPersonal.Artifacts[2].Integrity.Scope != "metadata" {
		t.Fatalf("got integrity scope %q", withPersonal.Artifacts[2].Integrity.Scope)
	}
}

func TestMCPIntegrityExcludesSecretConfigurationValues(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".mcp.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"test":{"env":{"TOKEN":"first"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	found := inventory.Inventory{Artifacts: []inventory.Artifact{{
		ID:     "mcp:portable-mcp:project:test",
		Name:   "test",
		Kind:   "mcp-server",
		Path:   path,
		Source: "portable-mcp",
		Scope:  "project",
		Metadata: map[string]string{
			"transport":             "stdio",
			"environment_variables": "TOKEN",
		},
	}}}

	first, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"test":{"env":{"TOKEN":"second"}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if first.Artifacts[0].Integrity.Scope != "metadata" {
		t.Fatalf("got scope %q", first.Artifacts[0].Integrity.Scope)
	}
	if first.Artifacts[0].Integrity.Digest != second.Artifacts[0].Integrity.Digest {
		t.Fatal("secret value changed MCP metadata integrity")
	}

	found.Artifacts[0].Metadata["command"] = "different"
	changedMetadata, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if changedMetadata.Artifacts[0].Integrity.Digest == first.Artifacts[0].Integrity.Digest {
		t.Fatal("sanitized MCP metadata change did not affect integrity")
	}
}

func TestContentDigestTracksOnlyExecutablePermission(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "SKILL.md")
	if err := os.WriteFile(path, []byte("# Review\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	found := inventory.Inventory{Artifacts: []inventory.Artifact{{
		ID:     "skill:review",
		Name:   "review",
		Kind:   "agent-skill",
		Path:   path,
		Source: root,
		Scope:  "project",
	}}}

	initial, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	nonExecutable, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if nonExecutable.Artifacts[0].Integrity.Digest != initial.Artifacts[0].Integrity.Digest {
		t.Fatal("digest changed for permissions not represented by Git")
	}

	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := Generate(found, Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if executable.Artifacts[0].Integrity.Digest == initial.Artifacts[0].Integrity.Digest {
		t.Fatal("digest did not change when executable status changed")
	}
}

func TestWriteAndCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "sourceward.lock.json")
	locked := Lockfile{
		SchemaVersion: SchemaVersion,
		Artifacts: []Artifact{{
			ID:     "skill:review",
			Name:   "review",
			Kind:   "agent-skill",
			Scope:  "project",
			Source: ".github/skills",
			Integrity: Integrity{
				Scope:     "content",
				Algorithm: "sha256",
				Digest:    "abc",
			},
		}},
	}

	if err := Write(path, locked); err != nil {
		t.Fatal(err)
	}
	if err := Check(path, locked); err != nil {
		t.Fatal(err)
	}

	changed := locked
	changed.Artifacts = append([]Artifact(nil), locked.Artifacts...)
	changed.Artifacts[0].Integrity.Digest = "def"
	if err := Check(path, changed); !errors.Is(err, ErrDrift) {
		t.Fatalf("got %v, want ErrDrift", err)
	}
}

func TestCheckRejectsUnsupportedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sourceward.lock.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":99,"artifacts":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err := Check(path, Lockfile{SchemaVersion: SchemaVersion, Artifacts: []Artifact{}})
	if err == nil {
		t.Fatal("expected an error")
	}
}
