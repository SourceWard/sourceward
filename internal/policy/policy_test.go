package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SourceWard/sourceward/internal/audit"
	"github.com/SourceWard/sourceward/internal/inventory"
)

func TestLoadRejectsUnknownFieldsAndSecrets(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "unknown", content: "version: 1\nunknown_field: true\n", want: "field unknown_field not found"},
		{name: "secret", content: "version: 1\napproved_sources:\n  - https://user:credential@example.invalid/repo\n", want: "must not contain credential"},
		{name: "broad exception", content: "version: 1\nexceptions:\n  - rule: SW001\n    reason: no artifact\n", want: "requires rule, artifact, and reason"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "sourceward.yaml"), []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(root)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestEvaluateAppliesExactExceptionAndFailsExpiredClosed(t *testing.T) {
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	configured := &Policy{
		Version: 1,
		Exceptions: []Exception{
			{Rule: "SW101", Artifact: "mcp:allowed", Reason: "Reviewed", Expires: "2026-09-16"},
			{Rule: "SW102", Artifact: "mcp:expired", Reason: "Temporary", Expires: "2026-09-14"},
		},
	}
	findings := []audit.Finding{
		{RuleID: "SW101", ArtifactID: "mcp:allowed", Severity: "high"},
		{RuleID: "SW101", ArtifactID: "mcp:other", Severity: "high"},
		{RuleID: "SW102", ArtifactID: "mcp:expired", Severity: "high"},
	}

	active, result := Evaluate(configured, inventory.Inventory{}, findings, t.TempDir(), now)
	if len(active) != 2 {
		t.Fatalf("got active findings %#v", active)
	}
	if len(result.AppliedExceptions) != 1 ||
		result.AppliedExceptions[0].Artifact != "mcp:allowed" {
		t.Fatalf("unexpected exceptions %#v", result.AppliedExceptions)
	}
	if len(result.Violations) != 1 ||
		result.Violations[0].Code != "exception_expired" ||
		result.Violations[0].ArtifactID != "mcp:expired" {
		t.Fatalf("unexpected violations %#v", result.Violations)
	}
}

func TestEvaluateDeterministicInventoryPolicy(t *testing.T) {
	root := t.TempDir()
	configured := &Policy{
		Version:              1,
		SeverityThreshold:    "medium",
		AllowedArtifactTypes: []string{"agent-skill"},
		ApprovedPublishers:   []string{"approved"},
		ApprovedSources:      []string{"https://example.invalid/approved.git"},
		DeniedRules:          []string{"SW005"},
		RequireLockfile:      true,
	}
	found := inventory.Inventory{Artifacts: []inventory.Artifact{
		{
			ID: "extension:z", Kind: "ide-extension", Source: "visual-studio-code",
			Provenance: inventory.Provenance{Publisher: "unapproved"},
		},
		{
			ID: "skill:a", Kind: "agent-skill",
			Provenance: inventory.Provenance{Repository: "https://example.invalid/unapproved.git"},
		},
	}}
	findings := []audit.Finding{{RuleID: "SW005", ArtifactID: "skill:a", Severity: "medium"}}

	firstFindings, first := Evaluate(configured, found, findings, root, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	secondFindings, second := Evaluate(configured, found, findings, root, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if !reflect.DeepEqual(firstFindings, secondFindings) || !reflect.DeepEqual(first, second) {
		t.Fatal("policy evaluation is not deterministic")
	}
	if len(first.Violations) != 6 {
		t.Fatalf("unexpected violations %#v", first.Violations)
	}
}
