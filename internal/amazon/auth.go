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
		"openid.pape.max_auth_age":         {"0"},
		"accountStatusPolicy":              {"P1"},
		"openid.assoc_handle":              {"amzn_device_na"},
		"pageId":                           {"amzn_device_common_dark"},
		"disableLoginPrepopulate":          {"1"},
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
