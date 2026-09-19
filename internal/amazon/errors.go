package amazon

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
)

// ErrSessionRejected means Amazon explicitly rejected the saved device credentials.
var ErrSessionRejected = errors.New("Amazon rejected the saved session; run 'kindlecli login' to authenticate again")

// APIError deliberately omits response bodies, which may contain credentials.
type APIError struct {
	Operation       string
	StatusCode      int
	sessionRejected bool
}

func (e *APIError) Error() string {
	if e.sessionRejected {
		return fmt.Sprintf("%s (HTTP %d): %s", e.Operation, e.StatusCode, ErrSessionRejected)
	}
	return fmt.Sprintf("%s failed (HTTP %d)", e.Operation, e.StatusCode)
}

func (e *APIError) Is(target error) bool {
	return target == ErrSessionRejected && e.sessionRejected
}

func responseError(operation string, status int, body []byte, deviceAuth bool) error {
	var result struct {
		Message string `json:"Message"`
	}
	_ = json.Unmarshal(body, &result)
	return &APIError{Operation: operation, StatusCode: status,
		sessionRejected: deviceAuth && (status == http.StatusUnauthorized ||
			(status == http.StatusForbidden && result.Message == "Failed to validate DeviceInfoToken.")),
	}
}

// NetworkError retains the cause for errors.Is/As without exposing signed URLs.
type NetworkError struct {
	Operation string
	Err       error
}

func (e *NetworkError) Error() string {
	var timeout net.Error
	if errors.As(e.Err, &timeout) && timeout.Timeout() {
		return e.Operation + ": request timed out"
	}
	return e.Operation + ": network request failed"
}
func (e *NetworkError) Unwrap() error { return e.Err }

const maxResponseBytes = 4 << 20

func readResponseBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading Amazon response failed")
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("Amazon response exceeds size limit")
	}
	return body, nil
}
