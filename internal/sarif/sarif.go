package sarif

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/SourceWard/sourceward/internal/audit"
)

const (
	schemaURL = "https://json.schemastore.org/sarif-2.1.0.json"
	version   = "2.1.0"
)

type Options struct {
	Root        string
	ToolVersion string
}

type Log struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []Run  `json:"runs"`
}

type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

type Tool struct {
	Driver Driver `json:"driver"`
}

type Driver struct {
	Name            string                `json:"name"`
	InformationURI  string                `json:"informationUri"`
	SemanticVersion string                `json:"semanticVersion"`
	Rules           []ReportingDescriptor `json:"rules"`
}

type ReportingDescriptor struct {
	ID                   string                 `json:"id"`
	ShortDescription     Message                `json:"shortDescription"`
	DefaultConfiguration ReportingConfiguration `json:"defaultConfiguration"`
}

type ReportingConfiguration struct {
	Level string `json:"level"`
}

type Result struct {
	RuleID     string           `json:"ruleId"`
	RuleIndex  int              `json:"ruleIndex"`
	Level      string           `json:"level"`
	Message    Message          `json:"message"`
	Locations  []Location       `json:"locations,omitempty"`
	Properties ResultProperties `json:"properties"`
}

type Message struct {
	Text string `json:"text"`
}

type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           Region           `json:"region"`
}

type ArtifactLocation struct {
	URI string `json:"uri"`
}

type Region struct {
	StartLine int `json:"startLine"`
}

type ResultProperties struct {
	ArtifactID         string `json:"sourcewardArtifactId"`
	SourceWardSeverity string `json:"sourcewardSeverity"`
}

func Write(writer io.Writer, findings []audit.Finding, options Options) error {
	log, err := Generate(findings, options)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(log); err != nil {
		return fmt.Errorf("encode SARIF: %w", err)
	}
	return nil
}

func Generate(findings []audit.Finding, options Options) (Log, error) {
	root, err := filepath.Abs(options.Root)
	if err != nil {
		return Log{}, fmt.Errorf("resolve repository root: %w", err)
	}

	sortedFindings := append([]audit.Finding(nil), findings...)
	sort.Slice(sortedFindings, func(i, j int) bool {
		left := sortedFindings[i]
		right := sortedFindings[j]
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.ArtifactID < right.ArtifactID
	})

	rules := rulesFromCatalog(audit.Rules())
	ruleIndexes := make(map[string]int, len(rules))
	for index, rule := range rules {
		ruleIndexes[rule.ID] = index
	}

	results := make([]Result, 0, len(sortedFindings))
	for _, finding := range sortedFindings {
		ruleIndex, exists := ruleIndexes[finding.RuleID]
		if !exists {
			return Log{}, fmt.Errorf("finding references unknown rule %s", finding.RuleID)
		}
		result := Result{
			RuleID:    finding.RuleID,
			RuleIndex: ruleIndex,
			Level:     level(finding.Severity),
			Message:   Message{Text: finding.Description},
			Properties: ResultProperties{
				ArtifactID:         finding.ArtifactID,
				SourceWardSeverity: finding.Severity,
			},
		}
		locationPath := finding.Path
		if finding.LocalPath != "" {
			locationPath = finding.LocalPath
		}
		if uri, ok := repositoryURI(root, locationPath); ok {
			result.Locations = []Location{{
				PhysicalLocation: PhysicalLocation{
					ArtifactLocation: ArtifactLocation{URI: uri},
					Region:           Region{StartLine: max(finding.Line, 1)},
				},
			}}
		}
		results = append(results, result)
	}

	return Log{
		Version: version,
		Schema:  schemaURL,
		Runs: []Run{{
			Tool: Tool{Driver: Driver{
				Name:            "SourceWard",
				InformationURI:  "https://github.com/SourceWard/sourceward",
				SemanticVersion: options.ToolVersion,
				Rules:           rules,
			}},
			Results: results,
		}},
	}, nil
}

func rulesFromCatalog(catalog []audit.Rule) []ReportingDescriptor {
	rules := make([]ReportingDescriptor, 0, len(catalog))
	for _, candidate := range catalog {
		rules = append(rules, ReportingDescriptor{
			ID:               candidate.ID,
			ShortDescription: Message{Text: candidate.Description},
			DefaultConfiguration: ReportingConfiguration{
				Level: level(candidate.Severity),
			},
		})
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].ID < rules[j].ID
	})
	return rules
}

func repositoryURI(root, path string) (string, bool) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	relative, err := filepath.Rel(root, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	uri := url.URL{Path: filepath.ToSlash(relative)}
	return uri.String(), true
}

func level(severity string) string {
	switch severity {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	case "low":
		return "note"
	default:
		return "none"
	}
}
