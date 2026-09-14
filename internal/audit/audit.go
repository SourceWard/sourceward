package audit

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/SourceWard/sourceward/internal/inventory"
)

type Finding struct {
	RuleID      string `json:"rule_id"`
	Severity    string `json:"severity"`
	ArtifactID  string `json:"artifact_id"`
	Path        string `json:"path"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Evidence    string `json:"evidence"`
}

type rule struct {
	id          string
	severity    string
	description string
	pattern     *regexp.Regexp
}

var rules = []rule{
	{
		id:          "SW001",
		severity:    "critical",
		description: "Remote content is piped directly to a shell",
		pattern:     regexp.MustCompile(`(?i)(curl|wget)\b[^\n|]*\|\s*(ba)?sh\b`),
	},
	{
		id:          "SW002",
		severity:    "high",
		description: "Skill references a sensitive credential location",
		pattern:     regexp.MustCompile(`(?i)(~/|\$HOME/)?\.(ssh|aws|azure|kube|config/gh)\b`),
	},
	{
		id:          "SW003",
		severity:    "high",
		description: "Skill references a likely secret-bearing environment variable",
		pattern:     regexp.MustCompile(`(?i)\b[A-Z][A-Z0-9_]*(TOKEN|SECRET|PASSWORD|PRIVATE_KEY)\b`),
	},
	{
		id:          "SW004",
		severity:    "medium",
		description: "Skill automatically allows every available tool",
		pattern:     regexp.MustCompile(`(?im)^\s*allowed-tools\s*:\s*["']?\*["']?\s*$`),
	},
	{
		id:          "SW005",
		severity:    "medium",
		description: "Skill contains hidden or bidirectional Unicode control characters",
		pattern:     regexp.MustCompile("[\u200B\u200C\u200D\u202A-\u202E\u2066-\u2069\uFEFF]"),
	},
}

func Audit(found inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	for _, artifact := range found.Artifacts {
		if artifact.Kind != "agent-skill" || artifact.Path == "" {
			continue
		}
		content, err := os.ReadFile(artifact.Path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", artifact.Path, err)
		}
		for _, candidate := range rules {
			for _, location := range candidate.pattern.FindAllIndex(content, -1) {
				line := 1 + strings.Count(string(content[:location[0]]), "\n")
				evidence := strings.TrimSpace(string(content[location[0]:location[1]]))
				findings = append(findings, Finding{
					RuleID:      candidate.id,
					Severity:    candidate.severity,
					ArtifactID:  artifact.ID,
					Path:        artifact.Path,
					Line:        line,
					Description: candidate.description,
					Evidence:    evidence,
				})
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity == findings[j].Severity {
			if findings[i].Path == findings[j].Path {
				return findings[i].Line < findings[j].Line
			}
			return findings[i].Path < findings[j].Path
		}
		return severityRank(findings[i].Severity) < severityRank(findings[j].Severity)
	})
	return findings, nil
}

func severityRank(severity string) int {
	switch severity {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	default:
		return 3
	}
}
