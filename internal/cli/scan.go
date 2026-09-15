package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/SourceWard/sourceward/internal/audit"
	"github.com/SourceWard/sourceward/internal/discovery"
	"github.com/SourceWard/sourceward/internal/inventory"
	"github.com/SourceWard/sourceward/internal/lockfile"
	"github.com/SourceWard/sourceward/internal/policy"
	"github.com/SourceWard/sourceward/internal/sarif"
)

func (application Application) runScan(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	format := flags.String("format", "table", "output format: table, json, or sarif")
	failOn := flags.String("fail-on", "high", "minimum finding severity: critical, high, medium, low, none")
	checkLock := flags.Bool("check-lock", false, "compare discovered artifacts with sourceward.lock.json")
	includePersonal := flags.Bool("include-personal", false, "include personal artifacts in lockfile comparison")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%w: scan does not accept positional arguments", ErrInvalidInput)
	}
	if err := validateFormat(*format, "table", "json", "sarif"); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if !validThreshold(*failOn) {
		return fmt.Errorf("%w: fail-on must be critical, high, medium, low, or none", ErrInvalidInput)
	}
	configuredPolicy, err := policy.Load(*root)
	if err != nil {
		if errors.Is(err, policy.ErrInvalidPolicy) {
			return fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		return err
	}

	found, err := application.discover(ctx, discovery.Options{Root: *root})
	if err != nil {
		return err
	}
	findings, err := application.audit(found)
	if err != nil {
		return err
	}
	findings, policyResult := policy.Evaluate(configuredPolicy, found, findings, *root, time.Now())
	effectiveThreshold := *failOn
	if configuredPolicy != nil && configuredPolicy.SeverityThreshold != "" {
		effectiveThreshold = configuredPolicy.SeverityThreshold
	}

	lockDiff, err := scanLockDiff(found, configuredPolicy, *root, *checkLock, *includePersonal)
	if err != nil {
		return err
	}
	if err := writeScanOutput(stdout, stderr, *format, *root, found, findings, policyResult, lockDiff); err != nil {
		return err
	}
	if len(policyResult.Violations) != 0 {
		return fmt.Errorf("%w: scan violates repository policy", ErrPolicyViolation)
	}
	if lockDiff != nil && !lockDiff.Clean() {
		return fmt.Errorf("%w: %v", lockfile.ErrDrift, lockfile.ErrDrift)
	}
	if shouldFail(findings, effectiveThreshold) {
		return fmt.Errorf("%w: scan found issues at or above %s severity", ErrFindingsPresent, effectiveThreshold)
	}
	return nil
}

func scanLockDiff(
	found inventory.Inventory,
	configured *policy.Policy,
	root string,
	checkLock,
	includePersonal bool,
) (*lockfile.Diff, error) {
	required := configured != nil && configured.RequireLockfile
	if !checkLock && !required {
		return nil, nil
	}
	path := filepath.Join(root, "sourceward.lock.json")
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) && required && !checkLock {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect lockfile: %w", err)
	}
	existing, err := lockfile.Read(path)
	if err != nil {
		return nil, err
	}
	current, err := lockfile.Generate(found, lockfile.Options{
		Root: root, IncludePersonal: includePersonal,
	})
	if err != nil {
		return nil, err
	}
	compared := lockfile.Compare(existing, current)
	return &compared, nil
}

func writeScanOutput(
	stdout,
	stderr io.Writer,
	format,
	root string,
	found inventory.Inventory,
	findings []audit.Finding,
	policyResult policy.Result,
	lockDiff *lockfile.Diff,
) error {
	if format == "json" {
		return writeJSON(stdout, struct {
			Inventory inventory.Inventory `json:"inventory"`
			Findings  []audit.Finding     `json:"findings"`
			Policy    policy.Result       `json:"policy"`
			Lockfile  *lockfile.Diff      `json:"lockfile,omitempty"`
		}{Inventory: found, Findings: findings, Policy: policyResult, Lockfile: lockDiff})
	}
	if format == "sarif" {
		return sarif.Write(stdout, findings, sarif.Options{Root: root, ToolVersion: Version})
	}

	writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "SEVERITY\tRULE\tARTIFACT\tLOCATION\tDESCRIPTION")
	for _, finding := range findings {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s:%d\t%s\n",
			finding.Severity, finding.RuleID, finding.ArtifactID,
			finding.Path, finding.Line, finding.Description)
	}
	if len(findings) == 0 {
		fmt.Fprintln(writer, "none\t-\t-\t-\tNo findings")
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	if err := writeDiagnostics(stderr, found.Diagnostics); err != nil {
		return err
	}
	for _, violation := range policyResult.Violations {
		fmt.Fprintf(stdout, "POLICY\t%s\t%s\t%s\n",
			violation.Code, violation.ArtifactID, violation.Message)
	}
	if lockDiff != nil {
		return writeLockDiff(stdout, *lockDiff, "table")
	}
	return nil
}
