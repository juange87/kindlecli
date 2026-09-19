package amazon

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSessionRejectionClassification(t *testing.T) {
	for _, tt := range []struct {
		status           int
		body             string
		device, rejected bool
	}{
		{401, "secret", true, true},
		{403, `{"Message":"Failed to validate DeviceInfoToken."}`, true, true},
		{403, `{"Message":"Access denied: secret"}`, true, false},
		{500, "secret", true, false},
		{401, "secret", false, false},
	} {
		err := fmt.Errorf("wrapped: %w", responseError("test", tt.status, []byte(tt.body), tt.device))
		if errors.Is(err, ErrSessionRejected) != tt.rejected {
			t.Errorf("classification for %d: %v", tt.status, err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("response body leaked")
		}
	}
}
