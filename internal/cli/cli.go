package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/SourceWard/sourceward/internal/audit"
	"github.com/SourceWard/sourceward/internal/discovery"
	"github.com/SourceWard/sourceward/internal/inventory"
	"github.com/SourceWard/sourceward/internal/lockfile"
)

const Version = "0.1.0-dev"

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
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, Version)
		return nil
	case "help", "--help", "-h":
		printHelp(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (application Application) runLock(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("lock", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	output := flags.String("output", "sourceward.lock.json", "lockfile path")
	check := flags.Bool("check", false, "verify the lockfile instead of writing it")
	includePersonal := flags.Bool("include-personal", false, "include personal skills and editor extensions")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("lock does not accept positional arguments")
	}

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
		if err := lockfile.Check(*output, locked); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Lockfile is current: %s\n", *output)
		return nil
	}
	if err := lockfile.Write(*output, locked); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Wrote %d artifacts to %s\n", len(locked.Artifacts), *output)
	return nil
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
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("discover does not accept positional arguments")
	}
	if err := validateFormat(*format); err != nil {
		return err
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
	return writer.Flush()
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
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("audit does not accept positional arguments")
	}
	if err := validateFormat(*format); err != nil {
		return err
	}
	if !validThreshold(*failOn) {
		return errors.New("fail-on must be critical, high, medium, low, or none")
	}

	found, err := application.discover(ctx, discovery.Options{Root: *root})
	if err != nil {
		return err
	}
	findings, err := application.audit(found)
	if err != nil {
		return err
	}

	if *format == "json" {
		if err := writeJSON(stdout, map[string]any{"findings": findings}); err != nil {
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

	if shouldFail(findings, *failOn) {
		return fmt.Errorf("audit found issues at or above %s severity", *failOn)
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

func validateFormat(format string) error {
	if format != "table" && format != "json" {
		return errors.New("format must be table or json")
	}
	return nil
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
  sourceward audit [--root PATH] [--format table|json] [--fail-on SEVERITY]
  sourceward lock [--root PATH] [--output PATH] [--check] [--include-personal]
  sourceward version`)
}
