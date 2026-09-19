package amazon

import (
	"encoding/json"
	"errors"
	"fmt"
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
