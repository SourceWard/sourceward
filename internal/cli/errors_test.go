package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/SourceWard/sourceward/internal/lockfile"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "success", want: ExitSuccess},
		{name: "invalid", err: fmt.Errorf("%w: flag", ErrInvalidInput), want: ExitInvalid},
		{name: "operational", err: errors.New("read failed"), want: ExitOperational},
		{name: "findings", err: fmt.Errorf("%w: high", ErrFindingsPresent), want: ExitFindings},
		{name: "policy", err: fmt.Errorf("%w: denied", ErrPolicyViolation), want: ExitPolicy},
		{name: "drift", err: fmt.Errorf("%w: changed", lockfile.ErrDrift), want: ExitDrift},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ExitCode(test.err); got != test.want {
				t.Fatalf("ExitCode(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}
