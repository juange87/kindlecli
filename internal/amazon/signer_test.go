// internal/amazon/signer_test.go
package amazon

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
)

func TestSignerDigestHeaderFormat(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	s := &Signer{
		PrivateKey: key,
		ADPToken:   "test-adp-token",
	}

	header := s.DigestHeaderForRequest("POST", "/TestPath", `{"key":"value"}`, "2024-01-01T00:00:00Z")

	// Format: base64_signature:date
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("expected format 'sig:date', got %q", header)
	}
	if parts[1] != "2024-01-01T00:00:00Z" {
		t.Errorf("date part = %q, want %q", parts[1], "2024-01-01T00:00:00Z")
	}
	// Base64 signature for 2048-bit key should be 344 chars
	if len(parts[0]) != 344 {
		t.Errorf("signature length = %d, want 344", len(parts[0]))
	}
}

func TestSignerDeterministic(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	s := &Signer{PrivateKey: key, ADPToken: "token"}

	h1 := s.DigestHeaderForRequest("GET", "/path", "", "2024-01-01T00:00:00Z")
	h2 := s.DigestHeaderForRequest("GET", "/path", "", "2024-01-01T00:00:00Z")
	if h1 != h2 {
		t.Error("same inputs produced different signatures")
	}
}

func TestSignerDifferentInputs(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	s := &Signer{PrivateKey: key, ADPToken: "token"}

	h1 := s.DigestHeaderForRequest("GET", "/path1", "", "2024-01-01T00:00:00Z")
	h2 := s.DigestHeaderForRequest("GET", "/path2", "", "2024-01-01T00:00:00Z")
	if h1 == h2 {
		t.Error("different inputs produced same signatures")
	}
}

func TestGetSigningDate(t *testing.T) {
	date := getSigningDate()
	if !strings.HasSuffix(date, "Z") {
		t.Errorf("signing date should end with Z, got %q", date)
	}
	if len(date) != 20 {
		t.Errorf("signing date length = %d, want 20 (ISO 8601)", len(date))
	}
}
