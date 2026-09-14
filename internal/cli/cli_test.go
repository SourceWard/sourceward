package cli

import (
	"bytes"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := Run([]string{"version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != Version+"\n" {
		t.Fatalf("got %q", stdout.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	if err := Run([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAuditRejectsInvalidThreshold(t *testing.T) {
	err := Run([]string{"audit", "--fail-on", "urgent"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error")
	}
}
