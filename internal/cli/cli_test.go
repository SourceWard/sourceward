package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SourceWard/sourceward/internal/audit"
	"github.com/SourceWard/sourceward/internal/discovery"
	"github.com/SourceWard/sourceward/internal/inventory"
)

func TestHelpAndVersionCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no arguments", want: "Usage:"},
		{name: "help command", args: []string{"help"}, want: "sourceward discover"},
		{name: "long help flag", args: []string{"--help"}, want: "sourceward audit"},
		{name: "version command", args: []string{"version"}, want: Version},
		{name: "version flag", args: []string{"--version"}, want: Version},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := testApplication(nil, nil).Run(test.args, &stdout, &bytes.Buffer{}); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stdout.String(), test.want) {
				t.Fatalf("output %q does not contain %q", stdout.String(), test.want)
			}
		})
	}
}

func TestSubcommandHelpReturnsSuccess(t *testing.T) {
	for _, command := range []string{"discover", "audit", "lock"} {
		t.Run(command, func(t *testing.T) {
			var stderr bytes.Buffer
			if err := testApplication(nil, nil).Run([]string{command, "--help"}, &bytes.Buffer{}, &stderr); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(stderr.String(), "Usage of "+command) {
				t.Fatalf("unexpected help output %q", stderr.String())
			}
		})
	}
}

func TestDiscoverJSON(t *testing.T) {
	found := inventory.Inventory{Artifacts: []inventory.Artifact{{
		ID:      "skill:review",
		Name:    "review",
		Kind:    "agent-skill",
		Version: "1.0.0",
		Scope:   "project",
		Source:  ".github/skills",
	}}}
	var stdout bytes.Buffer

	err := testApplication(&found, nil).Run(
		[]string{"discover", "--root", "/repo", "--format", "json"},
		&stdout,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}

	var output inventory.Inventory
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Artifacts) != 1 || output.Artifacts[0].ID != "skill:review" {
		t.Fatalf("unexpected inventory %#v", output)
	}
}

func TestDiscoverTable(t *testing.T) {
	found := inventory.Inventory{Artifacts: []inventory.Artifact{{
		ID:     "visual-studio-code:github.copilot",
		Name:   "github.copilot",
		Kind:   "ide-extension",
		Scope:  "personal",
		Source: "visual-studio-code",
	}}}
	var stdout bytes.Buffer

	if err := testApplication(&found, nil).Run([]string{"discover"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"KIND", "github.copilot", "visual-studio-code"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("output %q does not contain %q", stdout.String(), expected)
		}
	}
}

func TestAuditOutputsFindingsBeforeThresholdFailure(t *testing.T) {
	found := inventory.Inventory{}
	findings := []audit.Finding{{
		RuleID:      "SW001",
		Severity:    "critical",
		ArtifactID:  "skill:unsafe",
		Path:        "/repo/.github/skills/unsafe/SKILL.md",
		Line:        4,
		Description: "Remote content is piped directly to a shell",
	}}
	var stdout bytes.Buffer

	err := testApplication(&found, findings).Run(
		[]string{"audit", "--format", "json", "--fail-on", "high"},
		&stdout,
		&bytes.Buffer{},
	)
	if err == nil {
		t.Fatal("expected threshold failure")
	}
	if !strings.Contains(stdout.String(), `"rule_id": "SW001"`) {
		t.Fatalf("finding was not written before failure: %q", stdout.String())
	}
}

func TestAuditCanReportWithoutFailing(t *testing.T) {
	findings := []audit.Finding{{RuleID: "SW005", Severity: "medium"}}
	if err := testApplication(nil, findings).Run(
		[]string{"audit", "--fail-on", "none"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); err != nil {
		t.Fatal(err)
	}
}

func TestCommandValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown command", args: []string{"unknown"}, want: "unknown command"},
		{name: "discover format", args: []string{"discover", "--format", "yaml"}, want: "format must be"},
		{name: "discover positional argument", args: []string{"discover", "extra"}, want: "positional"},
		{name: "audit format", args: []string{"audit", "--format", "yaml"}, want: "format must be"},
		{name: "audit threshold", args: []string{"audit", "--fail-on", "urgent"}, want: "fail-on must be"},
		{name: "audit positional argument", args: []string{"audit", "extra"}, want: "positional"},
		{name: "lock positional argument", args: []string{"lock", "extra"}, want: "positional"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := testApplication(nil, nil).Run(test.args, &bytes.Buffer{}, &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got error %v, want one containing %q", err, test.want)
			}
		})
	}
}

func TestLockWritesAndChecksFile(t *testing.T) {
	output := filepath.Join(t.TempDir(), "sourceward.lock.json")
	found := inventory.Inventory{Artifacts: []inventory.Artifact{{
		ID:      "visual-studio-code:github.copilot",
		Name:    "github.copilot",
		Kind:    "ide-extension",
		Version: "1.0.0",
		Scope:   "project",
		Source:  "visual-studio-code",
	}}}
	application := testApplication(&found, nil)

	if err := application.Run(
		[]string{"lock", "--root", t.TempDir(), "--output", output},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); err != nil {
		t.Fatal(err)
	}
	if err := application.Run(
		[]string{"lock", "--root", t.TempDir(), "--output", output, "--check"},
		&bytes.Buffer{},
		&bytes.Buffer{},
	); err != nil {
		t.Fatal(err)
	}
}

func TestResolveLockfilePath(t *testing.T) {
	if got := resolveLockfilePath("/repo", ""); got != filepath.Join("/repo", "sourceward.lock.json") {
		t.Fatalf("got %q", got)
	}
	if got := resolveLockfilePath("/repo", "custom.json"); got != "custom.json" {
		t.Fatalf("got %q", got)
	}
}

func TestDiscoveryErrorIsReturned(t *testing.T) {
	expected := errors.New("discovery failed")
	application := New()
	application.discover = func(context.Context, discovery.Options) (inventory.Inventory, error) {
		return inventory.Inventory{}, expected
	}

	err := application.Run([]string{"discover"}, &bytes.Buffer{}, &bytes.Buffer{})
	if !errors.Is(err, expected) {
		t.Fatalf("got %v, want %v", err, expected)
	}
}

func testApplication(found *inventory.Inventory, findings []audit.Finding) Application {
	application := New()
	application.discover = func(context.Context, discovery.Options) (inventory.Inventory, error) {
		if found == nil {
			return inventory.Inventory{}, nil
		}
		return *found, nil
	}
	application.audit = func(inventory.Inventory) ([]audit.Finding, error) {
		return findings, nil
	}
	return application
}
