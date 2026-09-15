package sarif

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/SourceWard/sourceward/internal/audit"
)

func TestGenerateMapsFindingsToSARIF(t *testing.T) {
	root := t.TempDir()
	findings := []audit.Finding{
		{
			RuleID:      "SW005",
			Severity:    "medium",
			ArtifactID:  "skill:review",
			Path:        filepath.Join(root, ".github", "skills", "review", "SKILL.md"),
			Line:        8,
			Description: "Hidden Unicode control",
		},
		{
			RuleID:      "SW001",
			Severity:    "critical",
			ArtifactID:  "skill:install",
			Path:        filepath.Join(root, ".github", "skills", "install", "SKILL.md"),
			Line:        4,
			Description: "Remote content is piped directly to a shell",
		},
	}

	log, err := Generate(findings, Options{Root: root, ToolVersion: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("unexpected log %#v", log)
	}
	driver := log.Runs[0].Tool.Driver
	if driver.Name != "SourceWard" || driver.SemanticVersion != "0.1.0" {
		t.Fatalf("unexpected driver %#v", driver)
	}
	if len(driver.Rules) != 5 || driver.Rules[0].ID != "SW001" {
		t.Fatalf("rules are not deterministic: %#v", driver.Rules)
	}
	results := log.Runs[0].Results
	if len(results) != 2 || results[0].RuleID != "SW001" {
		t.Fatalf("results are not deterministic: %#v", results)
	}
	if results[0].Level != "error" {
		t.Fatalf("got level %q", results[0].Level)
	}
	location := results[0].Locations[0].PhysicalLocation
	if location.ArtifactLocation.URI != ".github/skills/install/SKILL.md" {
		t.Fatalf("got URI %q", location.ArtifactLocation.URI)
	}
	if location.Region.StartLine != 4 {
		t.Fatalf("got line %d", location.Region.StartLine)
	}
}

func TestGenerateOmitsExternalLocations(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "SKILL.md")
	log, err := Generate([]audit.Finding{{
		RuleID:      "SW002",
		Severity:    "high",
		ArtifactID:  "skill:personal",
		Path:        external,
		Line:        1,
		Description: "Sensitive credential location",
	}}, Options{Root: root, ToolVersion: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}

	result := log.Runs[0].Results[0]
	if len(result.Locations) != 0 {
		t.Fatalf("external path was exposed: %#v", result.Locations)
	}
	if result.Properties.ArtifactID != "skill:personal" {
		t.Fatalf("artifact identity was lost: %#v", result.Properties)
	}
}

func TestGenerateEscapesRepositoryURI(t *testing.T) {
	root := t.TempDir()
	log, err := Generate([]audit.Finding{{
		RuleID:      "SW001",
		Severity:    "critical",
		ArtifactID:  "skill:unsafe",
		Path:        filepath.Join(root, "skills", "unsafe install", "SKILL.md"),
		Line:        1,
		Description: "Remote content is piped directly to a shell",
	}}, Options{Root: root, ToolVersion: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}

	uri := log.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI
	if uri != "skills/unsafe%20install/SKILL.md" {
		t.Fatalf("got URI %q", uri)
	}
}

func TestGenerateRejectsUnknownRule(t *testing.T) {
	_, err := Generate([]audit.Finding{{
		RuleID:      "UNKNOWN",
		Severity:    "high",
		ArtifactID:  "skill:test",
		Description: "Unknown rule",
	}}, Options{Root: t.TempDir(), ToolVersion: "0.1.0"})
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestWriteProducesValidJSONForNoFindings(t *testing.T) {
	var output bytes.Buffer
	if err := Write(&output, nil, Options{Root: t.TempDir(), ToolVersion: "0.1.0"}); err != nil {
		t.Fatal(err)
	}

	var log Log
	if err := json.Unmarshal(output.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	if log.Runs[0].Results == nil || len(log.Runs[0].Tool.Driver.Rules) != 5 {
		t.Fatal("empty SARIF collections must be encoded as arrays")
	}
}

func TestSeverityLevels(t *testing.T) {
	tests := map[string]string{
		"critical": "error",
		"high":     "error",
		"medium":   "warning",
		"low":      "note",
		"unknown":  "none",
	}
	for severity, expected := range tests {
		if got := level(severity); got != expected {
			t.Fatalf("level(%q) = %q, want %q", severity, got, expected)
		}
	}
}
