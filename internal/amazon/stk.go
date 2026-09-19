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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	stkBase  = "https://stkservice.amazon.com"
	firsBase = "https://firs-ta-g7g.amazon.com"
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
	// UploadTimeout overrides the default 10-minute upload deadline.
	UploadTimeout time.Duration
}

// NewClient creates a Client from DeviceInfo.
func NewClient(info DeviceInfo) (*Client, error) {
	if strings.TrimSpace(info.ADPToken) == "" {
		return nil, fmt.Errorf("missing device authentication token")
	}
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
	if !stat.Mode().IsRegular() || stat.Size() == 0 {
		return "", fmt.Errorf("expected a nonempty regular file")
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
	u, err := url.Parse(uploadURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("Amazon returned an invalid HTTPS upload URL")
	}

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
	req.Header.Set("Accept-Language", "en-US,*")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	uploadClient := *httpClient
	uploadClient.Timeout = 10 * time.Minute
	if c.UploadTimeout > 0 {
		uploadClient.Timeout = c.UploadTimeout
	}
	resp, err := uploadClient.Do(req)
	if err != nil {
		return &NetworkError{Operation: "upload", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("upload", resp.StatusCode, nil, false)
	}
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	return err
}

func (c *Client) sendToKindle(stkToken string, deviceSerials []string, title, author, format string) (SendToKindleResponse, error) {
	body, err := c.stkRequest("/SendToKindle", map[string]any{
		"DocumentMetadata": map[string]any{
			"author":      author,
			"crc32":       0,
			"inputFormat": format,
			"title":       title,
		},
		"archive":           true,
		"deliveryMechanism": "WIFI",
		"outputFormat":      "MOBI",
		"stkToken":          stkToken,
		"targetDevices":     deviceSerials,
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

	resp, err := httpClient.Do(req)
	if err != nil {
		return &NetworkError{Operation: "device logout", Err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return responseError("device logout", resp.StatusCode, nil, false)
	}
	_, err = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBytes))
	return err
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
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ADP-Request-Digest", digest)
	req.Header.Set("X-ADP-Authentication-Token", c.Signer.ADPToken)
	req.Header.Set("Accept-Language", "en-US,*")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, &NetworkError{Operation: path, Err: err}
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, responseError(path, resp.StatusCode, body, true)
	}
	return body, nil
}
