# kindlecli Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI that sends EPUBs/PDFs to Kindle via Amazon's internal Send-to-Kindle API (OAuth2 + direct upload).

**Architecture:** Port of [stkclient](https://github.com/maxdjohnson/stkclient) (Python) to Go. OAuth2 PKCE login → device registration → RSA-signed API requests → file upload to S3 → SendToKindle dispatch. CLI built with Cobra.

**Tech Stack:** Go 1.25, Cobra (CLI), crypto/rsa + math/big (signing), net/http (API), encoding/xml + encoding/json (parsing)

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`

**Step 1: Initialize Go module and install Cobra**

Run:
```bash
cd /Users/juange/Documents/development/kindlecli
go mod init github.com/juange/kindlecli
go get github.com/spf13/cobra@latest
```

**Step 2: Create main.go**

```go
// main.go
package main

import "github.com/juange/kindlecli/cmd"

func main() {
	cmd.Execute()
}
```

**Step 3: Create cmd/root.go**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgDir string
var verbose bool

var rootCmd = &cobra.Command{
	Use:   "kindlecli",
	Short: "Send files to your Kindle",
	Long:  "A CLI tool for sending EPUBs and PDFs to your Kindle devices via Amazon's Send-to-Kindle service.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgDir, "config", "", "config directory (default: ~/.config/kindlecli)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
```

**Step 4: Verify it builds**

Run: `go build -o kindlecli . && ./kindlecli --help`
Expected: Help output showing "Send files to your Kindle"

**Step 5: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go
git commit -m "feat: project scaffolding with cobra root command"
```

---

### Task 2: Data Models

**Files:**
- Create: `internal/amazon/models.go`
- Create: `internal/amazon/models_test.go`

**Step 1: Write the tests**

```go
// internal/amazon/models_test.go
package amazon

import (
	"testing"
)

func TestDeviceInfoFromXML(t *testing.T) {
	xml := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<response>
	<device_private_key>-----BEGIN RSA PRIVATE KEY-----
MIIBogIBAAJBALRiMLAA
-----END RSA PRIVATE KEY-----</device_private_key>
	<adp_token>token123</adp_token>
	<device_type>A1K6D1WRW0MALS</device_type>
	<given_name>Juan</given_name>
	<name>Juan Garcia</name>
	<account_pool>Amazon</account_pool>
	<user_directed_id>uid123</user_directed_id>
	<user_device_name>Kindlecli</user_device_name>
	<home_region>NA</home_region>
</response>`)

	info, err := DeviceInfoFromXML(xml)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.ADPToken != "token123" {
		t.Errorf("got adp_token=%q, want %q", info.ADPToken, "token123")
	}
	if info.GivenName != "Juan" {
		t.Errorf("got given_name=%q, want %q", info.GivenName, "Juan")
	}
	if info.HomeRegion != "NA" {
		t.Errorf("got home_region=%q, want %q", info.HomeRegion, "NA")
	}
}

func TestGetOwnedDevicesResponseFromJSON(t *testing.T) {
	data := []byte(`{
		"ownedDevices": [
			{
				"deviceCapabilities": {"supportedFormats": true},
				"deviceName": "Kindle Paperwhite",
				"deviceSerialNumber": "G000XX123456"
			}
		],
		"statusCode": 0
	}`)

	resp, err := ParseGetOwnedDevicesResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.OwnedDevices) != 1 {
		t.Fatalf("got %d devices, want 1", len(resp.OwnedDevices))
	}
	if resp.OwnedDevices[0].DeviceSerialNumber != "G000XX123456" {
		t.Errorf("got serial=%q, want %q", resp.OwnedDevices[0].DeviceSerialNumber, "G000XX123456")
	}
}

func TestGetUploadUrlResponseFromJSON(t *testing.T) {
	data := []byte(`{
		"expiryTime": 1234567890,
		"statusCode": 0,
		"stkToken": "stk-abc123",
		"uploadUrl": "https://s3.amazonaws.com/upload/path"
	}`)

	resp, err := ParseGetUploadUrlResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.STKToken != "stk-abc123" {
		t.Errorf("got stkToken=%q, want %q", resp.STKToken, "stk-abc123")
	}
	if resp.UploadURL != "https://s3.amazonaws.com/upload/path" {
		t.Errorf("got uploadUrl=%q, want %q", resp.UploadURL, "https://s3.amazonaws.com/upload/path")
	}
}

func TestSendToKindleResponseFromJSON(t *testing.T) {
	data := []byte(`{"sku": "sku123", "statusCode": 0}`)

	resp, err := ParseSendToKindleResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SKU != "sku123" {
		t.Errorf("got sku=%q, want %q", resp.SKU, "sku123")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/amazon/ -v -run TestDevice`
Expected: FAIL — types and functions not defined

**Step 3: Implement models**

```go
// internal/amazon/models.go
package amazon

import (
	"encoding/json"
	"encoding/xml"
)

// DeviceInfo holds credentials returned by Amazon device registration.
type DeviceInfo struct {
	DevicePrivateKey string `json:"device_private_key" xml:"device_private_key"`
	ADPToken         string `json:"adp_token" xml:"adp_token"`
	DeviceType       string `json:"device_type" xml:"device_type"`
	GivenName        string `json:"given_name" xml:"given_name"`
	Name             string `json:"name" xml:"name"`
	AccountPool      string `json:"account_pool" xml:"account_pool"`
	UserDirectedID   string `json:"user_directed_id" xml:"user_directed_id"`
	UserDeviceName   string `json:"user_device_name" xml:"user_device_name"`
	HomeRegion       string `json:"home_region,omitempty" xml:"home_region"`
}

func DeviceInfoFromXML(data []byte) (DeviceInfo, error) {
	var wrapper struct {
		XMLName xml.Name `xml:"response"`
		DeviceInfo
	}
	if err := xml.Unmarshal(data, &wrapper); err != nil {
		return DeviceInfo{}, err
	}
	return wrapper.DeviceInfo, nil
}

// OwnedDevice represents a Kindle device.
type OwnedDevice struct {
	DeviceName         string `json:"deviceName"`
	DeviceSerialNumber string `json:"deviceSerialNumber"`
}

type GetOwnedDevicesResponse struct {
	OwnedDevices []OwnedDevice `json:"ownedDevices"`
	StatusCode   int           `json:"statusCode"`
}

func ParseGetOwnedDevicesResponse(data []byte) (GetOwnedDevicesResponse, error) {
	var resp GetOwnedDevicesResponse
	err := json.Unmarshal(data, &resp)
	return resp, err
}

type GetUploadUrlResponse struct {
	ExpiryTime int64  `json:"expiryTime"`
	StatusCode int    `json:"statusCode"`
	STKToken   string `json:"stkToken"`
	UploadURL  string `json:"uploadUrl"`
}

func ParseGetUploadUrlResponse(data []byte) (GetUploadUrlResponse, error) {
	var resp GetUploadUrlResponse
	err := json.Unmarshal(data, &resp)
	return resp, err
}

type SendToKindleResponse struct {
	SKU        string `json:"sku"`
	StatusCode int    `json:"statusCode"`
}

func ParseSendToKindleResponse(data []byte) (SendToKindleResponse, error) {
	var resp SendToKindleResponse
	err := json.Unmarshal(data, &resp)
	return resp, err
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/amazon/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/amazon/models.go internal/amazon/models_test.go
git commit -m "feat: add Amazon API data models with XML/JSON parsing"
```

---

### Task 3: RSA Request Signer

**Files:**
- Create: `internal/amazon/signer.go`
- Create: `internal/amazon/signer_test.go`

This is the most critical and testable component. Amazon's API requires a custom RSA signature scheme (PKCS#1 v1.5 padding without DigestInfo, applied directly with the private key exponent).

**Step 1: Write the test**

We generate a known RSA key and verify the signer produces deterministic, valid output.

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/amazon/ -v -run TestSigner`
Expected: FAIL — Signer type not defined

**Step 3: Implement the signer**

```go
// internal/amazon/signer.go
package amazon

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"math/big"
	"strings"
	"time"
)

// Signer implements RSA request signing for Amazon's Send-to-Kindle API.
type Signer struct {
	PrivateKey *rsa.PrivateKey
	ADPToken   string
}

// DigestHeaderForRequest computes the X-ADP-Request-Digest header value.
// If signingDate is empty, the current UTC time is used.
func (s *Signer) DigestHeaderForRequest(method, path, postData, signingDate string) string {
	if signingDate == "" {
		signingDate = getSigningDate()
	}

	sigData := strings.Join([]string{method, path, signingDate, postData, s.ADPToken}, "\n")
	hash := sha256.Sum256([]byte(sigData))

	sig := rsaSignCustomPadding(s.PrivateKey, hash[:])

	return base64.StdEncoding.EncodeToString(sig) + ":" + signingDate
}

// rsaSignCustomPadding replicates stkclient's signing scheme:
// PKCS#1 v1.5 type 1 padding without DigestInfo prefix.
// Block: 0x01 || 0xFF * (keySize - hashLen - 2) || 0x00 || hash
// Then raw RSA: c = m^d mod n
func rsaSignCustomPadding(key *rsa.PrivateKey, hash []byte) []byte {
	keySize := key.Size() // bytes (256 for 2048-bit)

	padded := make([]byte, keySize)
	padded[0] = 0x01
	psLen := keySize - len(hash) - 2
	for i := 1; i <= psLen; i++ {
		padded[i] = 0xFF
	}
	padded[psLen+1] = 0x00
	copy(padded[psLen+2:], hash)

	m := new(big.Int).SetBytes(padded)
	c := new(big.Int).Exp(m, key.D, key.N)

	result := make([]byte, keySize)
	cBytes := c.Bytes()
	copy(result[keySize-len(cBytes):], cBytes)

	return result
}

func getSigningDate() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/amazon/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/amazon/signer.go internal/amazon/signer_test.go
git commit -m "feat: implement RSA request signer for Amazon API"
```

---

### Task 4: Config / Session Management

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write the tests**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/juange/kindlecli/internal/amazon"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	info := amazon.DeviceInfo{
		DevicePrivateKey: "-----BEGIN RSA PRIVATE KEY-----\ntest\n-----END RSA PRIVATE KEY-----",
		ADPToken:         "adp-token-123",
		GivenName:        "Juan",
		DeviceType:       "A1K6D1WRW0MALS",
		Name:             "Juan Garcia",
		AccountPool:      "Amazon",
		UserDirectedID:   "uid",
		UserDeviceName:   "Kindlecli",
	}

	err := Save(dir, info)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file permissions
	stat, _ := os.Stat(filepath.Join(dir, "auth.json"))
	if stat.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %o, want 0600", stat.Mode().Perm())
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ADPToken != info.ADPToken {
		t.Errorf("ADPToken = %q, want %q", loaded.ADPToken, info.ADPToken)
	}
	if loaded.GivenName != info.GivenName {
		t.Errorf("GivenName = %q, want %q", loaded.GivenName, info.GivenName)
	}
}

func TestLoadNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for missing auth.json")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()

	info := amazon.DeviceInfo{ADPToken: "test"}
	_ = Save(dir, info)
	err := Delete(dir)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = Load(dir)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestDefaultDir(t *testing.T) {
	dir := DefaultDir()
	if dir == "" {
		t.Error("DefaultDir returned empty string")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/config/ -v`
Expected: FAIL — package/functions not defined

**Step 3: Implement config**

```go
// internal/config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/juange/kindlecli/internal/amazon"
)

const authFile = "auth.json"

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "kindlecli")
}

func Save(dir string, info amazon.DeviceInfo) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	path := filepath.Join(dir, authFile)
	return os.WriteFile(path, data, 0600)
}

func Load(dir string) (amazon.DeviceInfo, error) {
	path := filepath.Join(dir, authFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return amazon.DeviceInfo{}, fmt.Errorf("reading config: %w", err)
	}
	var info amazon.DeviceInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return amazon.DeviceInfo{}, fmt.Errorf("parsing config: %w", err)
	}
	return info, nil
}

func Delete(dir string) error {
	path := filepath.Join(dir, authFile)
	return os.Remove(path)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add config management for session persistence"
```

---

### Task 5: OAuth2 Authentication

**Files:**
- Create: `internal/amazon/auth.go`

No unit tests for this task — it calls external Amazon APIs. Tested manually via the login command.

**Step 1: Implement auth.go**

```go
// internal/amazon/auth.go
package amazon

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	clientID    = "658490dfb190e494030082836775981fa23be0c2425441860352ba0f55915b43002d"
	signinBase  = "https://www.amazon.com/ap/signin"
	tokenURL    = "https://api.amazon.com/auth/token"
	registerURL = "https://firs-ta-g7g.amazon.com/FirsProxy/registerDeviceWithToken"
)

type OAuth2 struct {
	verifier string
}

func NewOAuth2() *OAuth2 {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return &OAuth2{verifier: base64URLEncode(b)}
}

func (o *OAuth2) GetSignInURL() string {
	challenge := base64URLEncode(sha256Sum([]byte(o.verifier)))

	params := url.Values{
		"openid.claimed_id":           {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.ns.oa2":               {"http://www.amazon.com/ap/ext/oauth/2"},
		"openid.ns":                   {"http://specs.openid.net/auth/2.0"},
		"openid.identity":             {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.oa2.client_id":        {"device:" + clientID},
		"openid.mode":                 {"checkid_setup"},
		"openid.oa2.scope":            {"device_auth_access"},
		"openid.oa2.response_type":    {"code"},
		"openid.oa2.code_challenge":   {challenge},
		"openid.oa2.code_challenge_method": {"S256"},
		"openid.return_to":            {"https://www.amazon.com/gp/sendtokindle"},
		"openid.ns.pape":              {"http://specs.openid.net/extensions/pape/1.0"},
		"openid.pape.max_auth_age":    {"0"},
		"accountStatusPolicy":         {"P1"},
		"openid.assoc_handle":         {"amzn_device_na"},
		"pageId":                      {"amzn_device_common_dark"},
		"disableLoginPrepopulate":     {"1"},
	}

	return signinBase + "?" + params.Encode()
}

func (o *OAuth2) CreateClient(redirectURL string) (DeviceInfo, error) {
	code, err := parseAuthorizationCode(redirectURL)
	if err != nil {
		return DeviceInfo{}, err
	}
	accessToken, err := tokenExchange(code, o.verifier)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("token exchange: %w", err)
	}
	info, err := registerDevice(accessToken)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("device registration: %w", err)
	}
	return info, nil
}

func parseAuthorizationCode(redirectURL string) (string, error) {
	u, err := url.Parse(redirectURL)
	if err != nil {
		return "", fmt.Errorf("parsing redirect URL: %w", err)
	}
	code := u.Query().Get("openid.oa2.authorization_code")
	if code == "" {
		return "", fmt.Errorf("no authorization code found in URL")
	}
	return code, nil
}

func tokenExchange(authCode, codeVerifier string) (string, error) {
	body := map[string]string{
		"app_name":             "Unknown",
		"client_domain":        "DeviceLegacy",
		"client_id":            clientID,
		"code_algorithm":       "SHA-256",
		"code_verifier":        codeVerifier,
		"requested_token_type": "access_token",
		"source_token":         authCode,
		"source_token_type":    "authorization_code",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", tokenURL, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-US")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("x-amzn-identity-auth-domain", "api.amazon.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(b))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.AccessToken, nil
}

func registerDevice(accessToken string) (DeviceInfo, error) {
	xmlBody := fmt.Sprintf(`<?xml version='1.0' encoding='UTF-8'?>
<request><parameters><deviceType>A1K6D1WRW0MALS</deviceType><deviceSerialNumber>ZYSQ37GQ5JQDAIKDZ3WYH6I74MJCVEGG</deviceSerialNumber><pid>D21NN3GG</pid><authToken>%s</authToken><authTokenType>AccessToken</authTokenType><softwareVersion>253</softwareVersion><os_version>MacOSX_10.14.6_x64</os_version><device_model>KindleCLI</device_model></parameters></request>`, accessToken)

	req, _ := http.NewRequest("POST", registerURL, strings.NewReader(xmlBody))
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Expect", "")
	req.Header.Set("Accept-Language", "en-US,*")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return DeviceInfo{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DeviceInfo{}, err
	}

	if resp.StatusCode != 200 {
		return DeviceInfo{}, fmt.Errorf("device registration failed (%d): %s", resp.StatusCode, string(body))
	}

	return DeviceInfoFromXML(body)
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/amazon/`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/amazon/auth.go
git commit -m "feat: implement OAuth2 PKCE authentication with Amazon"
```

---

### Task 6: STK API Client

**Files:**
- Create: `internal/amazon/stk.go`

**Step 1: Implement the STK service client**

```go
// internal/amazon/stk.go
package amazon

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	stkBase    = "https://stkservice.amazon.com"
	firsBase   = "https://firs-ta-g7g.amazon.com"
)

var defaultClientInfo = map[string]string{
	"appName":        "ShellExtension",
	"appVersion":     "1.1.1.253",
	"os":             "MacOSX_10.14.6_x64",
	"osArchitecture": "x64",
}

// Client provides access to the Send-to-Kindle API.
type Client struct {
	Signer *Signer
}

// NewClient creates a Client from DeviceInfo.
func NewClient(info DeviceInfo) (*Client, error) {
	block, _ := pem.Decode([]byte(info.DevicePrivateKey))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM private key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing private key: %w", err)
	}
	return &Client{
		Signer: &Signer{
			PrivateKey: key,
			ADPToken:   info.ADPToken,
		},
	}, nil
}

// NewClientFromKey creates a Client from an existing RSA key and token.
func NewClientFromKey(key *rsa.PrivateKey, adpToken string) *Client {
	return &Client{
		Signer: &Signer{PrivateKey: key, ADPToken: adpToken},
	}
}

// GetOwnedDevices returns all Kindle devices on the account.
func (c *Client) GetOwnedDevices() ([]OwnedDevice, error) {
	body, err := c.stkRequest("/GetListOfOwnedDevices", map[string]any{})
	if err != nil {
		return nil, err
	}
	resp, err := ParseGetOwnedDevicesResponse(body)
	if err != nil {
		return nil, err
	}
	return resp.OwnedDevices, nil
}

// SendFile uploads a file and sends it to all given devices.
func (c *Client) SendFile(filePath string, deviceSerials []string, title, author string) (string, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	fileSize := stat.Size()

	// 1. Get upload URL
	uploadResp, err := c.getUploadURL(fileSize)
	if err != nil {
		return "", fmt.Errorf("getting upload URL: %w", err)
	}

	// 2. Upload file
	if err := c.uploadFile(uploadResp.UploadURL, filePath, fileSize); err != nil {
		return "", fmt.Errorf("uploading file: %w", err)
	}

	// 3. Send to Kindle
	ext := strings.ToUpper(strings.TrimPrefix(filepath.Ext(filePath), "."))
	resp, err := c.sendToKindle(uploadResp.STKToken, deviceSerials, title, author, ext)
	if err != nil {
		return "", fmt.Errorf("sending to kindle: %w", err)
	}

	return resp.SKU, nil
}

func (c *Client) getUploadURL(fileSize int64) (GetUploadUrlResponse, error) {
	body, err := c.stkRequest("/GetUploadUrl", map[string]any{
		"fileSize": fileSize,
	})
	if err != nil {
		return GetUploadUrlResponse{}, err
	}
	return ParseGetUploadUrlResponse(body)
}

func (c *Client) uploadFile(uploadURL, filePath string, fileSize int64) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	req, err := http.NewRequest("PUT", uploadURL, f)
	if err != nil {
		return err
	}
	req.ContentLength = fileSize
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Accept-Language", "en-US,*")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *Client) sendToKindle(stkToken string, deviceSerials []string, title, author, format string) (SendToKindleResponse, error) {
	body, err := c.stkRequest("/SendToKindle", map[string]any{
		"DocumentMetadata": map[string]any{
			"author":      author,
			"crc32":       0,
			"inputFormat": format,
			"title":       title,
		},
		"archive":            true,
		"deliveryMechanism":  "WIFI",
		"outputFormat":       "MOBI",
		"stkToken":           stkToken,
		"targetDevices":      deviceSerials,
	})
	if err != nil {
		return SendToKindleResponse{}, err
	}
	return ParseSendToKindleResponse(body)
}

// Logout terminates the Send-to-Kindle session.
func (c *Client) Logout() error {
	path := "/FirsProxy/disownFiona?contentDeleted=false"
	signingDate := getSigningDate()
	digest := c.Signer.DigestHeaderForRequest("GET", path, "", signingDate)

	req, _ := http.NewRequest("GET", firsBase+path, nil)
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("X-ADP-Request-Digest", digest)
	req.Header.Set("X-ADP-Authentication-Token", c.Signer.ADPToken)
	req.Header.Set("Accept-Language", "en-US,*")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)
	return nil
}

func (c *Client) stkRequest(path string, payload map[string]any) ([]byte, error) {
	payload["ClientInfo"] = defaultClientInfo

	jsonData, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		return nil, err
	}

	signingDate := getSigningDate()
	digest := c.Signer.DigestHeaderForRequest("POST", path, string(jsonData), signingDate)

	req, err := http.NewRequest("POST", stkBase+path, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ADP-Request-Digest", digest)
	req.Header.Set("X-ADP-Authentication-Token", c.Signer.ADPToken)
	req.Header.Set("Accept-Language", "en-US,*")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("STK API error (%d) %s: %s", resp.StatusCode, path, string(body))
	}
	return body, nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./internal/amazon/`
Expected: No errors

**Step 3: Commit**

```bash
git add internal/amazon/stk.go
git commit -m "feat: implement Send-to-Kindle API client (upload, send, devices, logout)"
```

---

### Task 7: Login Command

**Files:**
- Create: `cmd/login.go`

**Step 1: Implement the login command**

```go
// cmd/login.go
package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to your Amazon account",
	Long:  "Opens your browser for Amazon OAuth2 login. After signing in, paste the redirect URL back here.",
	RunE:  runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	// Check if already logged in
	if _, err := config.Load(dir); err == nil {
		fmt.Println("Already logged in. Use 'kindlecli logout' first to re-authenticate.")
		return nil
	}

	oauth := amazon.NewOAuth2()
	signinURL := oauth.GetSignInURL()

	fmt.Println("Opening browser for Amazon login...")
	if err := openBrowser(signinURL); err != nil {
		fmt.Println("Could not open browser. Please open this URL manually:")
		fmt.Println(signinURL)
	}

	fmt.Println()
	fmt.Print("After signing in, paste the redirect URL here: ")
	reader := bufio.NewReader(os.Stdin)
	redirectURL, _ := reader.ReadString('\n')
	redirectURL = strings.TrimSpace(redirectURL)

	if redirectURL == "" {
		return fmt.Errorf("no URL provided")
	}

	fmt.Println("Authenticating...")
	deviceInfo, err := oauth.CreateClient(redirectURL)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := config.Save(dir, deviceInfo); err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	name := deviceInfo.GivenName
	if name == "" {
		name = deviceInfo.Name
	}
	fmt.Printf("Logged in as %s. Session saved.\n", name)
	return nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}
	return cmd.Start()
}
```

**Step 2: Verify it builds**

Run: `go build -o kindlecli . && ./kindlecli login --help`
Expected: Help text for the login command

**Step 3: Commit**

```bash
git add cmd/login.go
git commit -m "feat: add login command with OAuth2 browser flow"
```

---

### Task 8: Send Command

**Files:**
- Create: `cmd/send.go`

**Step 1: Implement the send command**

```go
// cmd/send.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var sendTitle string
var sendAuthor string

var sendCmd = &cobra.Command{
	Use:   "send <file> [file...]",
	Short: "Send files to your Kindle",
	Long:  "Upload EPUB or PDF files and send them to all your Kindle devices.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSend,
}

func init() {
	sendCmd.Flags().StringVar(&sendTitle, "title", "", "document title (default: filename)")
	sendCmd.Flags().StringVar(&sendAuthor, "author", "", "document author")
	rootCmd.AddCommand(sendCmd)
}

var supportedFormats = map[string]bool{
	".epub": true,
	".pdf":  true,
	".mobi": true,
	".azw":  true,
	".azw3": true,
	".doc":  true,
	".docx": true,
	".txt":  true,
	".rtf":  true,
	".htm":  true,
	".html": true,
}

func runSend(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	deviceInfo, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("not logged in. Run 'kindlecli login' first")
	}

	client, err := amazon.NewClient(deviceInfo)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	// Get all devices
	devices, err := client.GetOwnedDevices()
	if err != nil {
		return fmt.Errorf("getting devices: %w", err)
	}
	if len(devices) == 0 {
		return fmt.Errorf("no Kindle devices found on your account")
	}

	serials := make([]string, len(devices))
	for i, d := range devices {
		serials[i] = d.DeviceSerialNumber
	}

	// Send each file
	for _, filePath := range args {
		ext := strings.ToLower(filepath.Ext(filePath))
		if !supportedFormats[ext] {
			fmt.Fprintf(os.Stderr, "Skipping %s: unsupported format %q\n", filePath, ext)
			continue
		}

		stat, err := os.Stat(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", filePath, err)
			continue
		}

		title := sendTitle
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
		}

		sizeMB := float64(stat.Size()) / 1024 / 1024
		fmt.Printf("Sending %s (%.1f MB)...\n", filepath.Base(filePath), sizeMB)

		_, err = client.SendFile(filePath, serials, title, sendAuthor)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send %s: %v\n", filePath, err)
			continue
		}
		fmt.Printf("Sent to %d device(s).\n", len(devices))
	}

	return nil
}
```

**Step 2: Verify it builds**

Run: `go build -o kindlecli . && ./kindlecli send --help`
Expected: Help text showing usage and flags

**Step 3: Commit**

```bash
git add cmd/send.go
git commit -m "feat: add send command for uploading files to Kindle"
```

---

### Task 9: Devices & Logout Commands

**Files:**
- Create: `cmd/devices.go`
- Create: `cmd/logout.go`

**Step 1: Implement devices command**

```go
// cmd/devices.go
package cmd

import (
	"fmt"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var devicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List your Kindle devices",
	RunE:  runDevices,
}

func init() {
	rootCmd.AddCommand(devicesCmd)
}

func runDevices(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	deviceInfo, err := config.Load(dir)
	if err != nil {
		return fmt.Errorf("not logged in. Run 'kindlecli login' first")
	}

	client, err := amazon.NewClient(deviceInfo)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	devices, err := client.GetOwnedDevices()
	if err != nil {
		return fmt.Errorf("getting devices: %w", err)
	}

	if len(devices) == 0 {
		fmt.Println("No Kindle devices found.")
		return nil
	}

	fmt.Printf("Found %d device(s):\n", len(devices))
	for i, d := range devices {
		fmt.Printf("  %d. %s (%s)\n", i+1, d.DeviceName, d.DeviceSerialNumber)
	}
	return nil
}
```

**Step 2: Implement logout command**

```go
// cmd/logout.go
package cmd

import (
	"fmt"

	"github.com/juange/kindlecli/internal/amazon"
	"github.com/juange/kindlecli/internal/config"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and delete saved session",
	RunE:  runLogout,
}

func init() {
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	dir := cfgDir
	if dir == "" {
		dir = config.DefaultDir()
	}

	deviceInfo, err := config.Load(dir)
	if err != nil {
		fmt.Println("Not logged in.")
		return nil
	}

	// Try to logout remotely (best effort)
	client, err := amazon.NewClient(deviceInfo)
	if err == nil {
		if err := client.Logout(); err != nil && verbose {
			fmt.Printf("Remote logout warning: %v\n", err)
		}
	}

	if err := config.Delete(dir); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	fmt.Println("Logged out successfully.")
	return nil
}
```

**Step 3: Verify everything builds**

Run: `go build -o kindlecli . && ./kindlecli --help`
Expected: Help output showing all four commands: login, send, devices, logout

**Step 4: Commit**

```bash
git add cmd/devices.go cmd/logout.go
git commit -m "feat: add devices and logout commands"
```

---

### Task 10: Build, Manual Test & Polish

**Step 1: Build the final binary**

Run:
```bash
go build -o kindlecli .
```

**Step 2: Test the login flow**

Run:
```bash
./kindlecli login
```

Expected:
1. Browser opens Amazon sign-in page
2. After signing in, Amazon redirects to amazon.com/gp/sendtokindle
3. Copy the URL from the browser address bar
4. Paste it into the terminal
5. CLI prints "Logged in as [Name]. Session saved."
6. Check `~/.config/kindlecli/auth.json` exists

**Step 3: Test listing devices**

Run: `./kindlecli devices`
Expected: List of your Kindle devices with names and serial numbers

**Step 4: Test sending a file**

Run: `./kindlecli send /path/to/test.epub`
Expected: "Sending test.epub (X.X MB)... Sent to N device(s)."

**Step 5: Test logout**

Run: `./kindlecli logout`
Expected: "Logged out successfully." and `~/.config/kindlecli/auth.json` deleted.

**Step 6: Fix any issues found during testing**

Iterate on any bugs discovered. Common issues to watch for:
- XML parsing of Amazon's registration response may have different field names
- RSA key format from Amazon might need `PKCS8` instead of `PKCS1`
- URL encoding order may matter for OAuth2 parameters

**Step 7: Final commit**

```bash
git add -A
git commit -m "chore: polish and finalize v1"
```

---

## Summary

| Task | Description | Estimated Complexity |
|------|-------------|---------------------|
| 1 | Project scaffolding | Low |
| 2 | Data models + tests | Low |
| 3 | RSA signer + tests | Medium (crypto) |
| 4 | Config management + tests | Low |
| 5 | OAuth2 authentication | Medium |
| 6 | STK API client | Medium |
| 7 | Login command | Low |
| 8 | Send command | Low |
| 9 | Devices + logout commands | Low |
| 10 | Build, test, iterate | Variable |
