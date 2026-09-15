package audit

import (
	"fmt"
	"os"
	"path/filepath"
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
	LocalPath   string `json:"-"`
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

type artifactRule struct {
	Rule
	kind        string
	metadataKey string
}

var skillRules = []compiledRule{
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

var artifactRules = []artifactRule{
	{Rule: Rule{ID: "SW101", Severity: "high", Description: "MCP server launches through a command shell"}, kind: "mcp-server", metadataKey: "risk_shell_execution"},
	{Rule: Rule{ID: "SW102", Severity: "high", Description: "MCP server executes a package without an immutable version"}, kind: "mcp-server", metadataKey: "risk_unpinned_package"},
	{Rule: Rule{ID: "SW103", Severity: "high", Description: "MCP configuration contains an inline credential value"}, kind: "mcp-server", metadataKey: "risk_inline_credentials"},
	{Rule: Rule{ID: "SW104", Severity: "medium", Description: "MCP server requests a broad filesystem root"}, kind: "mcp-server", metadataKey: "risk_broad_filesystem"},
	{Rule: Rule{ID: "SW105", Severity: "high", Description: "Remote MCP server uses unencrypted HTTP transport"}, kind: "mcp-server", metadataKey: "risk_insecure_transport"},
	{Rule: Rule{ID: "SW106", Severity: "medium", Description: "MCP server receives likely sensitive environment variables"}, kind: "mcp-server", metadataKey: "risk_sensitive_environment"},
	{Rule: Rule{ID: "SW201", Severity: "medium", Description: "Extension activates for every workspace event"}, kind: "ide-extension", metadataKey: "risk_broad_activation"},
	{Rule: Rule{ID: "SW202", Severity: "high", Description: "Extension package declares an installation lifecycle script"}, kind: "ide-extension", metadataKey: "risk_install_scripts"},
	{Rule: Rule{ID: "SW203", Severity: "medium", Description: "Extension depends on a process-execution package"}, kind: "ide-extension", metadataKey: "risk_process_dependencies"},
	{Rule: Rule{ID: "SW204", Severity: "medium", Description: "Extension depends on a network-capable package"}, kind: "ide-extension", metadataKey: "risk_network_dependencies"},
	{Rule: Rule{ID: "SW205", Severity: "medium", Description: "Extension depends on a code-obfuscation package"}, kind: "ide-extension", metadataKey: "risk_obfuscation_dependencies"},
}

func Rules() []Rule {
	catalog := make([]Rule, 0, len(skillRules)+len(artifactRules))
	for _, candidate := range skillRules {
		catalog = append(catalog, candidate.Rule)
	}
	for _, candidate := range artifactRules {
		catalog = append(catalog, candidate.Rule)
	}
	return catalog
}

func Audit(found inventory.Inventory) ([]Finding, error) {
	var findings []Finding
	for _, artifact := range found.Artifacts {
		if artifact.Kind == "agent-skill" && artifact.Path != "" {
			content, err := os.ReadFile(artifact.Path)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", artifact.Path, err)
			}
			for _, candidate := range skillRules {
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
		for _, candidate := range artifactRules {
			evidence, ok := artifact.Metadata[candidate.metadataKey]
			if artifact.Kind != candidate.kind || !ok || evidence == "" || evidence == "false" {
				continue
			}
			path := artifact.Path
			localPath := artifact.LocalPath
			if artifact.Kind == "ide-extension" {
				if path != "" {
					path = filepath.Join(path, "package.json")
				}
				if localPath != "" {
					localPath = filepath.Join(localPath, "package.json")
				}
			}
			findings = append(findings, Finding{
				RuleID:      candidate.ID,
				Severity:    candidate.Severity,
				ArtifactID:  artifact.ID,
				Path:        path,
				LocalPath:   localPath,
				Line:        1,
				Description: candidate.Description,
				Evidence:    evidence,
			})
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
