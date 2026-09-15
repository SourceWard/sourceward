package cli

import (
	"errors"

	"github.com/SourceWard/sourceward/internal/lockfile"
)

const (
	ExitSuccess     = 0
	ExitInvalid     = 2
	ExitOperational = 3
	ExitFindings    = 4
	ExitPolicy      = 5
	ExitDrift       = 6
)

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrFindingsPresent = errors.New("findings exceed severity threshold")
	ErrPolicyViolation = errors.New("repository policy violation")
)

func ExitCode(err error) int {
	switch {
	case err == nil:
		return ExitSuccess
	case errors.Is(err, ErrInvalidInput):
		return ExitInvalid
	case errors.Is(err, ErrFindingsPresent):
		return ExitFindings
	case errors.Is(err, ErrPolicyViolation):
		return ExitPolicy
	case errors.Is(err, lockfile.ErrDrift):
		return ExitDrift
	default:
		return ExitOperational
	}
}
