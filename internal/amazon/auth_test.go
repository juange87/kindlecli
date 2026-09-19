package amazon

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mockHTTP(t *testing.T, f roundTripFunc) {
	t.Helper()
	old := httpClient
	httpClient = &http.Client{Transport: f}
	t.Cleanup(func() { httpClient = old })
}
func mockResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestSignInReusesBrowserSessionAndKeepsPKCE(t *testing.T) {
	o := NewOAuth2()
	for _, fresh := range []bool{false, true} {
		u, err := url.Parse(o.SignInURL(fresh))
		if err != nil {
			t.Fatal(err)
		}
		q := u.Query()
		if q.Get("openid.oa2.code_challenge") != base64URLEncode(sha256Sum([]byte(o.Verifier))) {
			t.Fatal("bad PKCE challenge")
		}
		if q.Has("openid.pape.max_auth_age") != fresh || q.Has("disableLoginPrepopulate") != fresh {
			t.Fatal("unexpected fresh auth policy")
		}
		if strings.Contains(u.String(), o.Verifier) {
			t.Fatal("verifier in URL")
		}
	}
}

func TestRedirectValidation(t *testing.T) {
	for _, raw := range []string{
		"https://evil.example/gp/sendtokindle?openid.oa2.authorization_code=secret",
		"http://www.amazon.com/gp/sendtokindle?openid.oa2.authorization_code=secret",
		"https://www.amazon.com/other?openid.oa2.authorization_code=secret",
		"https://user@www.amazon.com/gp/sendtokindle?openid.oa2.authorization_code=secret",
		"https://www.amazon.com/gp/sendtokindle?openid.oa2.authorization_code=secret&openid.oa2.authorization_code=other",
		"https://www.amazon.com/gp/sendtokindle?openid.oa2.authorization_code=secret#fragment",
	} {
		if _, err := parseAuthorizationCode(raw); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("bad validation: %v", err)
		}
	}
	got, err := parseAuthorizationCode("https://www.amazon.com/gp/sendtokindle?openid.oa2.authorization_code=example")
	if err != nil || got != "example" {
		t.Fatal(got, err)
	}
}

func TestTokenExchangeRejectsEmptyAndRedactsErrors(t *testing.T) {
	for _, tt := range []struct {
		status int
		body   string
	}{{200, `{}`}, {400, `{"secret":"secret-value"}`}} {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			mockHTTP(t, func(r *http.Request) (*http.Response, error) { return mockResponse(tt.status, tt.body), nil })
			_, err := tokenExchange("code", "verifier")
			if err == nil || strings.Contains(err.Error(), "secret-value") {
				t.Fatalf("bad error: %v", err)
			}
		})
	}
}

func TestRegistrationRejectsIncompleteCredentials(t *testing.T) {
	mockHTTP(t, func(r *http.Request) (*http.Response, error) {
		return mockResponse(200, `<response><adp_token>token</adp_token></response>`), nil
	})
	if _, err := registerDevice("token"); err == nil {
		t.Fatal("accepted incomplete registration")
	}
}
