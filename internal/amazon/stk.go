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

	resp, err := httpClient.Do(req)
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

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, responseError(path, resp.StatusCode, body, true)
	}
	return body, nil
}
