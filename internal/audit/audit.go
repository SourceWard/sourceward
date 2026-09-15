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

type Rule struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}

type compiledRule struct {
	Rule
	pattern *regexp.Regexp
}

var rules = []compiledRule{
	{
		Rule: Rule{
			ID:          "SW001",
			Severity:    "critical",
			Description: "Remote content is piped directly to a shell",
		},
		pattern: regexp.MustCompile(`(?i)(curl|wget)\b[^\n|]*\|\s*(ba)?sh\b`),
	},
	{
		Rule: Rule{
			ID:          "SW002",
			Severity:    "high",
			Description: "Skill references a sensitive credential location",
		},
		pattern: regexp.MustCompile(`(?i)(~/|\$HOME/)?\.(ssh|aws|azure|kube|config/gh)\b`),
	},
	{
		Rule: Rule{
			ID:          "SW003",
			Severity:    "high",
			Description: "Skill references a likely secret-bearing environment variable",
		},
		pattern: regexp.MustCompile(`(?i)\b[A-Z][A-Z0-9_]*(TOKEN|SECRET|PASSWORD|PRIVATE_KEY)\b`),
	},
	{
		Rule: Rule{
			ID:          "SW004",
			Severity:    "medium",
			Description: "Skill automatically allows every available tool",
		},
		pattern: regexp.MustCompile(`(?im)^\s*allowed-tools\s*:\s*["']?\*["']?\s*$`),
	},
	{
		Rule: Rule{
			ID:          "SW005",
			Severity:    "medium",
			Description: "Skill contains hidden or bidirectional Unicode control characters",
		},
		pattern: regexp.MustCompile("[\u200B\u200C\u200D\u202A-\u202E\u2066-\u2069\uFEFF]"),
	},
}

func Rules() []Rule {
	catalog := make([]Rule, len(rules))
	for index, candidate := range rules {
		catalog[index] = candidate.Rule
	}
	return catalog
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
					RuleID:      candidate.ID,
					Severity:    candidate.Severity,
					ArtifactID:  artifact.ID,
					Path:        artifact.Path,
					Line:        line,
					Description: candidate.Description,
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
