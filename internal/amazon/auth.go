// internal/amazon/auth.go
package amazon

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

const (
	clientID    = "658490dfb190e494030082836775981fa23be0c2425441860352ba0f55915b43002d"
	signinBase  = "https://www.amazon.com/ap/signin"
	tokenURL    = "https://api.amazon.com/auth/token"
	registerURL = "https://firs-ta-g7g.amazon.com/FirsProxy/registerDeviceWithToken"
)

type OAuth2 struct {
	Verifier string `json:"verifier"`
}

func NewOAuth2() *OAuth2 {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return &OAuth2{Verifier: base64URLEncode(b)}
}

func (o *OAuth2) GetSignInURL() string { return o.SignInURL(false) }

// SignInURL allows Amazon to reuse browser authentication unless fresh is requested.
func (o *OAuth2) SignInURL(fresh bool) string {
	challenge := base64URLEncode(sha256Sum([]byte(o.Verifier)))

	params := url.Values{
		"openid.claimed_id":                {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.ns.oa2":                    {"http://www.amazon.com/ap/ext/oauth/2"},
		"openid.ns":                        {"http://specs.openid.net/auth/2.0"},
		"openid.identity":                  {"http://specs.openid.net/auth/2.0/identifier_select"},
		"openid.oa2.client_id":             {"device:" + clientID},
		"openid.mode":                      {"checkid_setup"},
		"openid.oa2.scope":                 {"device_auth_access"},
		"openid.oa2.response_type":         {"code"},
		"openid.oa2.code_challenge":        {challenge},
		"openid.oa2.code_challenge_method": {"S256"},
		"openid.return_to":                 {"https://www.amazon.com/gp/sendtokindle"},
		"openid.ns.pape":                   {"http://specs.openid.net/extensions/pape/1.0"},
		"accountStatusPolicy":              {"P1"},
		"openid.assoc_handle":              {"amzn_device_na"},
		"pageId":                           {"amzn_device_common_dark"},
	}

	if fresh {
		params.Set("openid.pape.max_auth_age", "0")
		params.Set("disableLoginPrepopulate", "1")
	}
	return signinBase + "?" + params.Encode()
}

// Verbose controls whether auth progress is logged to stderr.
var Verbose bool

func (o *OAuth2) CreateClient(redirectURL string) (DeviceInfo, error) {
	code, err := parseAuthorizationCode(redirectURL)
	if err != nil {
		return DeviceInfo{}, err
	}
	if Verbose {
		fmt.Fprintln(os.Stderr, "  Authorization code received.")
	}

	fmt.Fprintf(os.Stderr, "  Exchanging token...\n")
	accessToken, err := tokenExchange(code, o.Verifier)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("token exchange: %w", err)
	}
	if Verbose {
		fmt.Fprintf(os.Stderr, "  Access token received.\n")
	}

	fmt.Fprintf(os.Stderr, "  Registering device...\n")
	info, err := registerDevice(accessToken)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("device registration: %w", err)
	}
	return info, nil
}

func parseAuthorizationCode(redirectURL string) (string, error) {
	u, err := url.Parse(redirectURL)
	if err != nil {
		return "", fmt.Errorf("invalid redirect URL")
	}
	if u.Scheme != "https" || u.Host != "www.amazon.com" || u.Path != "/gp/sendtokindle" || u.User != nil || u.Fragment != "" {
		return "", fmt.Errorf("expected the HTTPS Amazon Send to Kindle redirect URL")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", fmt.Errorf("invalid redirect query")
	}
	codes := query["openid.oa2.authorization_code"]
	if len(codes) != 1 {
		return "", fmt.Errorf("expected exactly one authorization code")
	}
	code := codes[0]
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

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", &NetworkError{Operation: "token exchange", Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", responseError("token exchange", resp.StatusCode, nil, false)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("Amazon returned an empty access token")
	}
	return result.AccessToken, nil
}

func registerDevice(accessToken string) (DeviceInfo, error) {
	var escaped bytes.Buffer
	if err := xml.EscapeText(&escaped, []byte(accessToken)); err != nil {
		return DeviceInfo{}, err
	}
	xmlBody := fmt.Sprintf(`<?xml version='1.0' encoding='UTF-8'?>
<request><parameters><deviceType>A1K6D1WRW0MALS</deviceType><deviceSerialNumber>ZYSQ37GQ5JQDAIKDZ3WYH6I74MJCVEGG</deviceSerialNumber><pid>D21NN3GG</pid><authToken>%s</authToken><authTokenType>AccessToken</authTokenType><softwareVersion>253</softwareVersion><os_version>MacOSX_10.14.6_x64</os_version><device_model>KindleCLI</device_model></parameters></request>`, escaped.String())

	req, _ := http.NewRequest("POST", registerURL, strings.NewReader(xmlBody))
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Expect", "")
	req.Header.Set("Accept-Language", "en-US,*")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return DeviceInfo{}, &NetworkError{Operation: "device registration", Err: err}
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp.Body)
	if err != nil {
		return DeviceInfo{}, err
	}

	if resp.StatusCode != 200 {
		return DeviceInfo{}, responseError("device registration", resp.StatusCode, nil, false)
	}

	info, err := DeviceInfoFromXML(body)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("invalid device registration response")
	}
	if _, err := NewClient(info); err != nil {
		return DeviceInfo{}, fmt.Errorf("Amazon returned invalid device credentials")
	}
	return info, nil
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
