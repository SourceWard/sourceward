package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/SourceWard/sourceward/internal/audit"
	"github.com/SourceWard/sourceward/internal/discovery"
	"github.com/SourceWard/sourceward/internal/inventory"
	"github.com/SourceWard/sourceward/internal/lockfile"
	"github.com/SourceWard/sourceward/internal/policy"
	"github.com/SourceWard/sourceward/internal/sarif"
)

var Version = "0.1.0"

type Application struct {
	discover func(context.Context, discovery.Options) (inventory.Inventory, error)
	audit    func(inventory.Inventory) ([]audit.Finding, error)
}

func New() Application {
	return Application{
		discover: discovery.Discover,
		audit:    audit.Audit,
	}
}

func (application Application) Run(args []string, stdout, stderr io.Writer) error {
	ctx := context.Background()
	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}

	switch args[0] {
	case "discover":
		return application.runDiscover(ctx, args[1:], stdout, stderr)
	case "audit":
		return application.runAudit(ctx, args[1:], stdout, stderr)
	case "lock":
		return application.runLock(ctx, args[1:], stdout, stderr)
	case "diff":
		return application.runDiff(ctx, args[1:], stdout, stderr)
	case "scan":
		return application.runScan(ctx, args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, Version)
		return nil
	case "help", "--help", "-h":
		printHelp(stdout)
		return nil
	default:
		return fmt.Errorf("%w: unknown command %q", ErrInvalidInput, args[0])
	}
}

func (application Application) runLock(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("lock", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	output := flags.String("output", "", "lockfile path (default: ROOT/sourceward.lock.json)")
	check := flags.Bool("check", false, "verify the lockfile instead of writing it")
	includePersonal := flags.Bool("include-personal", false, "include personal skills and editor extensions")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%w: lock does not accept positional arguments", ErrInvalidInput)
	}
	lockPath := resolveLockfilePath(*root, *output)

	found, err := application.discover(ctx, discovery.Options{Root: *root})
	if err != nil {
		return err
	}
	locked, err := lockfile.Generate(found, lockfile.Options{
		Root:            *root,
		IncludePersonal: *includePersonal,
	})
	if err != nil {
		return err
	}

	if *check {
		existing, err := lockfile.Read(lockPath)
		if err != nil {
			return err
		}
		diff := lockfile.Compare(existing, locked)
		if !diff.Clean() {
			if err := writeLockDiff(stdout, diff, "table"); err != nil {
				return err
			}
			return fmt.Errorf("%w: %v", lockfile.ErrDrift, lockfile.ErrDrift)
		}
		fmt.Fprintf(stdout, "Lockfile is current: %s\n", lockPath)
		return nil
	}
	if err := lockfile.Write(lockPath, locked); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Wrote %d artifacts to %s\n", len(locked.Artifacts), lockPath)
	return nil
}

func (application Application) runDiff(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("diff", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	lockPath := flags.String("lockfile", "", "lockfile path (default: ROOT/sourceward.lock.json)")
	format := flags.String("format", "table", "output format: table or json")
	includePersonal := flags.Bool("include-personal", false, "include personal skills and editor extensions")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%w: diff does not accept positional arguments", ErrInvalidInput)
	}
	if err := validateFormat(*format, "table", "json"); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	path := resolveLockfilePath(*root, *lockPath)
	existing, err := lockfile.Read(path)
	if err != nil {
		return err
	}
	found, err := application.discover(ctx, discovery.Options{Root: *root})
	if err != nil {
		return err
	}
	current, err := lockfile.Generate(found, lockfile.Options{Root: *root, IncludePersonal: *includePersonal})
	if err != nil {
		return err
	}
	diff := lockfile.Compare(existing, current)
	if err := writeLockDiff(stdout, diff, *format); err != nil {
		return err
	}
	if !diff.Clean() {
		return fmt.Errorf("%w: %v", lockfile.ErrDrift, lockfile.ErrDrift)
	}
	return nil
}

func writeLockDiff(writer io.Writer, diff lockfile.Diff, format string) error {
	if format == "json" {
		return writeJSON(writer, diff)
	}
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "CHANGE\tARTIFACT\tSCOPE\tSOURCE\tFIELDS")
	for _, group := range []struct {
		name    string
		changes []lockfile.ArtifactChange
	}{
		{name: "added", changes: diff.Added},
		{name: "removed", changes: diff.Removed},
		{name: "changed", changes: diff.Changed},
	} {
		for _, change := range group.changes {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
				group.name, change.ID, change.Scope, change.Source, strings.Join(change.Fields, ","))
		}
	}
	if diff.Clean() {
		fmt.Fprintln(table, "clean\t-\t-\t-\t-")
	}
	return table.Flush()
}

func resolveLockfilePath(root, output string) string {
	if output != "" {
		return output
	}
	return filepath.Join(root, "sourceward.lock.json")
}

func (application Application) runDiscover(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("discover", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	format := flags.String("format", "table", "output format: table or json")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%w: discover does not accept positional arguments", ErrInvalidInput)
	}
	if err := validateFormat(*format, "table", "json"); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	found, err := application.discover(ctx, discovery.Options{Root: *root})
	if err != nil {
		return err
	}
	if *format == "json" {
		return writeJSON(stdout, found)
	}

	writer := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "KIND\tNAME\tVERSION\tSCOPE\tSOURCE")
	for _, artifact := range found.Artifacts {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n",
			artifact.Kind, artifact.Name, artifact.Version, artifact.Scope, artifact.Source)
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	return writeDiagnostics(stderr, found.Diagnostics)
}

func writeDiagnostics(writer io.Writer, diagnostics []inventory.Diagnostic) error {
	if len(diagnostics) == 0 {
		return nil
	}
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "\nDISCOVERY DIAGNOSTICS")
	fmt.Fprintln(table, "LEVEL\tPROVIDER\tCODE\tPATH\tMESSAGE")
	for _, diagnostic := range diagnostics {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			diagnostic.Level,
			diagnostic.Provider,
			diagnostic.Code,
			diagnostic.Path,
			diagnostic.Message,
		)
	}
	return table.Flush()
}

func (application Application) runAudit(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("audit", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	format := flags.String("format", "table", "output format: table or json")
	failOn := flags.String("fail-on", "high", "minimum severity that causes a non-zero exit: critical, high, medium, low, none")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}

		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%w: audit does not accept positional arguments", ErrInvalidInput)
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

	if *format == "json" {
		if err := writeJSON(stdout, struct {
			Findings []audit.Finding `json:"findings"`
			Policy   policy.Result   `json:"policy"`
		}{Findings: findings, Policy: policyResult}); err != nil {
			return err
		}
	} else if *format == "sarif" {
		if err := sarif.Write(stdout, findings, sarif.Options{Root: *root, ToolVersion: Version}); err != nil {
			return err
		}
	} else if *format == "table" {
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
	}

	if len(policyResult.Violations) != 0 {
		return fmt.Errorf("%w: audit violates repository policy", ErrPolicyViolation)
	}
	if shouldFail(findings, effectiveThreshold) {
		return fmt.Errorf("%w: audit found issues at or above %s severity", ErrFindingsPresent, effectiveThreshold)
	}
	return nil
}

func shouldFail(findings []audit.Finding, threshold string) bool {
	ranks := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "none": -1}
	limit := ranks[threshold]
	if threshold == "none" {
		return false
	}
	for _, finding := range findings {
		if ranks[finding.Severity] <= limit {
			return true
		}
	}
	return false
}

func validateFormat(format string, allowed ...string) error {
	for _, candidate := range allowed {
		if format == candidate {
			return nil
		}
	}
	return fmt.Errorf("format must be %s", strings.Join(allowed, ", "))
}

func validThreshold(threshold string) bool {
	switch threshold {
	case "critical", "high", "medium", "low", "none":
		return true
	default:
		return false
	}
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func printHelp(writer io.Writer) {
	fmt.Fprintln(writer, `SourceWard inventories and audits the capabilities developers and AI agents run.

Usage:
  sourceward discover [--root PATH] [--format table|json]
  sourceward audit [--root PATH] [--format table|json|sarif] [--fail-on SEVERITY]
  sourceward lock [--root PATH] [--output PATH] [--check] [--include-personal]
  sourceward diff [--root PATH] [--lockfile PATH] [--format table|json] [--include-personal]
  sourceward scan [--root PATH] [--format table|json|sarif] [--fail-on SEVERITY] [--check-lock]
  sourceward version`)
}
