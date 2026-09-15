package discovery

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SourceWard/sourceward/internal/inventory"
)

func TestGitProvenanceSupportsWorktreesNestedPathsAndDirtyState(t *testing.T) {
	repository := t.TempDir()
	runGitTest(t, repository, "init")
	runGitTest(t, repository, "config", "user.name", "SourceWard Test")
	runGitTest(t, repository, "config", "user.email", "sourceward@example.invalid")
	runGitTest(t, repository, "remote", "add", "origin", "https://account:credential-value@example.invalid/org/repository.git")

	skillPath := filepath.Join(repository, "skills", "review", "SKILL.md")
	writeTestFile(t, skillPath, "# Review\n")
	runGitTest(t, repository, "add", ".")
	runGitTest(t, repository, "commit", "-m", "add skill")

	worktree := filepath.Join(t.TempDir(), "worktree")
	runGitTest(t, repository, "worktree", "add", "-b", "provenance-test", worktree)
	worktreeSkill := filepath.Join(worktree, "skills", "review", "SKILL.md")
	artifact := inventory.Artifact{
		ID:   "skill:review",
		Kind: "agent-skill",
		Path: worktreeSkill,
	}

	clean := gitProvenance(context.Background(), artifact, execCommand)
	if clean.Kind != "git" {
		t.Fatalf("got provenance kind %q", clean.Kind)
	}
	if clean.Repository != "https://example.invalid/org/repository.git" {
		t.Fatalf("got sanitized repository %q", clean.Repository)
	}
	if clean.Revision == "" {
		t.Fatal("missing immutable revision")
	}
	if clean.Dirty == nil || *clean.Dirty {
		t.Fatalf("expected clean worktree, got %#v", clean.Dirty)
	}
	if clean.Subdir != "skills/review" {
		t.Fatalf("got subdirectory %q", clean.Subdir)
	}

	if err := os.WriteFile(worktreeSkill, []byte("# Changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirty := gitProvenance(context.Background(), artifact, execCommand)
	if dirty.Dirty == nil || !*dirty.Dirty {
		t.Fatalf("expected dirty worktree, got %#v", dirty.Dirty)
	}
	if dirty.Revision != clean.Revision {
		t.Fatalf("dirty state changed revision: %q != %q", dirty.Revision, clean.Revision)
	}
}

func TestGitProvenanceIsExplicitWhenAbsent(t *testing.T) {
	provenance := gitProvenance(context.Background(), inventory.Artifact{
		ID:   "skill:local",
		Kind: "agent-skill",
		Path: filepath.Join(t.TempDir(), "SKILL.md"),
	}, execCommand)
	if provenance.Kind != "unknown" {
		t.Fatalf("got provenance %#v", provenance)
	}
}

func TestSanitizeRepositoryURL(t *testing.T) {
	tests := map[string]string{
		"https://account:credential-value@example.invalid/org/repo.git?token=value#fragment": "https://example.invalid/org/repo.git",
		"ssh://git:credential-value@example.invalid/org/repo.git":                            "ssh://example.invalid/org/repo.git",
		"git@example.invalid:org/repo.git":                                                   "ssh://example.invalid/org/repo.git",
		"file:///private/source":                                                             "",
		"/private/source":                                                                    "",
	}
	for input, want := range tests {
		t.Run(strings.ReplaceAll(input, "/", "_"), func(t *testing.T) {
			if got := sanitizeRepositoryURL(input); got != want {
				t.Fatalf("sanitizeRepositoryURL(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func runGitTest(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
}
