package policy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/SourceWard/sourceward/internal/audit"
	"github.com/SourceWard/sourceward/internal/inventory"
	"gopkg.in/yaml.v3"
)

const Version = 1

type Policy struct {
	Version              int         `yaml:"version"`
	SeverityThreshold    string      `yaml:"severity_threshold,omitempty"`
	AllowedArtifactTypes []string    `yaml:"allowed_artifact_types,omitempty"`
	ApprovedPublishers   []string    `yaml:"approved_publishers,omitempty"`
	ApprovedSources      []string    `yaml:"approved_sources,omitempty"`
	DeniedRules          []string    `yaml:"denied_rules,omitempty"`
	RequireLockfile      bool        `yaml:"require_lockfile,omitempty"`
	Exceptions           []Exception `yaml:"exceptions,omitempty"`
}

type Exception struct {
	Rule     string `yaml:"rule" json:"rule"`
	Artifact string `yaml:"artifact" json:"artifact"`
	Reason   string `yaml:"reason" json:"reason"`
	Expires  string `yaml:"expires,omitempty" json:"expires,omitempty"`
}

type Result struct {
	Applied           bool        `json:"applied"`
	SeverityThreshold string      `json:"severity_threshold,omitempty"`
	Violations        []Violation `json:"violations"`
	AppliedExceptions []Exception `json:"applied_exceptions"`
}

type Violation struct {
	Code       string `json:"code"`
	ArtifactID string `json:"artifact_id,omitempty"`
	RuleID     string `json:"rule_id,omitempty"`
	Message    string `json:"message"`
}

var secretPattern = regexp.MustCompile(`(?i)(-----BEGIN [A-Z ]*PRIVATE KEY-----|(?:gh[pousr]_|sk-)[A-Za-z0-9_-]{8,}|bearer\s+[A-Za-z0-9._-]{8,}|://[^/@:\s]+:[^/@\s]+@)`)

func Load(root string) (*Policy, error) {
	path := filepath.Join(root, "sourceward.yaml")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read policy: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var configured Policy
	if err := decoder.Decode(&configured); err != nil {
		return nil, fmt.Errorf("decode sourceward.yaml: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("sourceward.yaml must contain one YAML document")
	}
	if err := configured.Validate(); err != nil {
		return nil, err
	}
	return &configured, nil
}

func (configured Policy) Validate() error {
	if configured.Version != Version {
		return fmt.Errorf("policy version must be %d", Version)
	}
	if configured.SeverityThreshold != "" && !validSeverity(configured.SeverityThreshold) {
		return errors.New("policy severity_threshold must be critical, high, medium, low, or none")
	}
	knownRules := map[string]struct{}{}
	for _, rule := range audit.Rules() {
		knownRules[rule.ID] = struct{}{}
	}
	for _, rule := range configured.DeniedRules {
		if _, ok := knownRules[rule]; !ok {
			return fmt.Errorf("policy denied_rules contains unknown rule %q", rule)
		}
	}
	for index, exception := range configured.Exceptions {
		if exception.Rule == "" || exception.Artifact == "" || strings.TrimSpace(exception.Reason) == "" {
			return fmt.Errorf("policy exception %d requires rule, artifact, and reason", index)
		}
		if _, ok := knownRules[exception.Rule]; !ok {
			return fmt.Errorf("policy exception %d references unknown rule %q", index, exception.Rule)
		}
		if exception.Expires != "" {
			if _, err := time.Parse(time.DateOnly, exception.Expires); err != nil {
				return fmt.Errorf("policy exception %d expires must use YYYY-MM-DD", index)
			}
		}
	}
	for _, value := range configured.allStrings() {
		if secretPattern.MatchString(value) {
			return errors.New("sourceward.yaml must not contain credential values")
		}
	}
	return nil
}

func Evaluate(
	configured *Policy,
	found inventory.Inventory,
	findings []audit.Finding,
	root string,
	now time.Time,
) ([]audit.Finding, Result) {
	if configured == nil {
		return findings, Result{Violations: []Violation{}, AppliedExceptions: []Exception{}}
	}
	result := Result{
		Applied:           true,
		SeverityThreshold: configured.SeverityThreshold,
		Violations:        []Violation{},
		AppliedExceptions: []Exception{},
	}

	allowedTypes := stringSet(configured.AllowedArtifactTypes)
	approvedPublishers := stringSet(configured.ApprovedPublishers)
	approvedSources := stringSet(configured.ApprovedSources)
	for _, artifact := range found.Artifacts {
		if len(allowedTypes) != 0 {
			if _, ok := allowedTypes[artifact.Kind]; !ok {
				result.Violations = append(result.Violations, Violation{
					Code: "artifact_type_not_allowed", ArtifactID: artifact.ID,
					Message: "artifact type is not allowed by repository policy",
				})
			}
		}
		if artifact.Kind == "ide-extension" && len(approvedPublishers) != 0 {
			if _, ok := approvedPublishers[artifact.Provenance.Publisher]; !ok {
				result.Violations = append(result.Violations, Violation{
					Code: "publisher_not_approved", ArtifactID: artifact.ID,
					Message: "extension publisher is not approved by repository policy",
				})
			}
		}
		if len(approvedSources) != 0 && !sourceApproved(artifact, approvedSources) {
			result.Violations = append(result.Violations, Violation{
				Code: "source_not_approved", ArtifactID: artifact.ID,
				Message: "artifact source is not approved by repository policy",
			})
		}
	}
	if configured.RequireLockfile {
		if _, err := os.Stat(filepath.Join(root, "sourceward.lock.json")); err != nil {
			result.Violations = append(result.Violations, Violation{
				Code: "lockfile_required", Message: "repository policy requires sourceward.lock.json",
			})
		}
	}

	active := make([]audit.Finding, 0, len(findings))
	denied := stringSet(configured.DeniedRules)
	for _, finding := range findings {
		exception, state := matchingException(configured.Exceptions, finding, now)
		if state == "active" {
			result.AppliedExceptions = append(result.AppliedExceptions, exception)
			continue
		}
		if state == "expired" {
			result.Violations = append(result.Violations, Violation{
				Code: "exception_expired", ArtifactID: finding.ArtifactID, RuleID: finding.RuleID,
				Message: "matching policy exception has expired",
			})
		}
		active = append(active, finding)
		if _, ok := denied[finding.RuleID]; ok {
			result.Violations = append(result.Violations, Violation{
				Code: "denied_rule", ArtifactID: finding.ArtifactID, RuleID: finding.RuleID,
				Message: "finding matches a rule denied by repository policy",
			})
		}
	}
	sortResults(&result)
	return active, result
}

func matchingException(exceptions []Exception, finding audit.Finding, now time.Time) (Exception, string) {
	for _, exception := range exceptions {
		if exception.Rule != finding.RuleID || exception.Artifact != finding.ArtifactID {
			continue
		}
		if exception.Expires == "" {
			return exception, "active"
		}
		expires, _ := time.Parse(time.DateOnly, exception.Expires)
		if now.After(expires.Add(24*time.Hour - time.Nanosecond)) {
			return exception, "expired"
		}
		return exception, "active"
	}
	return Exception{}, ""
}

func sourceApproved(artifact inventory.Artifact, approved map[string]struct{}) bool {
	for _, candidate := range []string{
		artifact.Source,
		artifact.Provenance.Repository,
		artifact.Provenance.Provider,
	} {
		if _, ok := approved[candidate]; ok && candidate != "" {
			return true
		}
	}
	return false
}

func sortResults(result *Result) {
	sort.Slice(result.Violations, func(i, j int) bool {
		left, right := result.Violations[i], result.Violations[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.ArtifactID != right.ArtifactID {
			return left.ArtifactID < right.ArtifactID
		}
		return left.RuleID < right.RuleID
	})
	sort.Slice(result.AppliedExceptions, func(i, j int) bool {
		left, right := result.AppliedExceptions[i], result.AppliedExceptions[j]
		if left.Rule != right.Rule {
			return left.Rule < right.Rule
		}
		return left.Artifact < right.Artifact
	})
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func validSeverity(value string) bool {
	switch value {
	case "critical", "high", "medium", "low", "none":
		return true
	default:
		return false
	}
}

func (configured Policy) allStrings() []string {
	values := append([]string{}, configured.AllowedArtifactTypes...)
	values = append(values, configured.ApprovedPublishers...)
	values = append(values, configured.ApprovedSources...)
	values = append(values, configured.DeniedRules...)
	for _, exception := range configured.Exceptions {
		values = append(values, exception.Rule, exception.Artifact, exception.Reason, exception.Expires)
	}
	return values
}
