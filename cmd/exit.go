package cmd

import (
	"errors"

	"github.com/juange87/kindlecli/internal/amazon"
)

// Exit codes are part of the automation contract (see README):
// 0 success, 1 other error, 2 login required, 3 Amazon could not be reached.
const (
	exitOK            = 0
	exitFailure       = 1
	exitLoginRequired = 2
	exitUnavailable   = 3
)

// codedError carries an explicit process exit code.
type codedError struct {
	code int
	err  error
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }

// loginRequired marks err as recoverable only through 'kindlecli login'.
func loginRequired(err error) error { return &codedError{exitLoginRequired, err} }

// exitCode maps an error to the documented exit codes. Explicit codes win;
// session rejection from any command means login is required, and a network
// failure means Amazon could not be reached.
func exitCode(err error) int {
	if err == nil {
		return exitOK
	}
	var coded *codedError
	if errors.As(err, &coded) {
		return coded.code
	}
	if errors.Is(err, amazon.ErrSessionRejected) {
		return exitLoginRequired
	}
	var network *amazon.NetworkError
	if errors.As(err, &network) {
		return exitUnavailable
	}
	return exitFailure
}

// statusExitCode maps an 'auth status' state to its exit code.
func statusExitCode(state string) int {
	switch state {
	case "valid":
		return exitOK
	case "missing", "invalid", "rejected":
		return exitLoginRequired
	case "unavailable":
		return exitUnavailable
	default:
		return exitFailure
	}
}
