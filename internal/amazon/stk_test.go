package amazon

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testClient(t *testing.T) *Client {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return NewClientFromKey(key, "test")
}

func TestUploadUsesIndependentDeadlineAndRedactsURL(t *testing.T) {
	file := filepath.Join(t.TempDir(), "book.epub")
	if err := os.WriteFile(file, []byte("book"), 0600); err != nil {
		t.Fatal(err)
	}
	mockHTTP(t, func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) < 19*time.Minute {
			t.Fatal("upload deadline too short")
		}
		if r.ContentLength != 4 {
			t.Fatal("incorrect content length")
		}
		return nil, &url.Error{Op: "Put", URL: r.URL.String(), Err: context.DeadlineExceeded}
	})
	client := testClient(t)
	client.UploadTimeout = 20 * time.Minute
	err := client.uploadFile("https://example.com/upload?secret=credential", file, 4)
	if err == nil || strings.Contains(err.Error(), "credential") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestLogoutReportsRejection(t *testing.T) {
	mockHTTP(t, func(r *http.Request) (*http.Response, error) { return mockResponse(403, "secret"), nil })
	if err := testClient(t).Logout(); err == nil {
		t.Fatal("logout falsely succeeded")
	}
}

func TestSTKDecompressesGzip(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "gzip" {
			t.Error("transport should negotiate gzip")
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		fmt.Fprint(gz, `{"statusCode":0,"ownedDevices":[]}`)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	mockHTTP(t, func(r *http.Request) (*http.Response, error) {
		clone := r.Clone(r.Context())
		clone.URL.Scheme = target.Scheme
		clone.URL.Host = target.Host
		return http.DefaultTransport.RoundTrip(clone)
	})
	if _, err := testClient(t).GetOwnedDevices(); err != nil {
		t.Fatal(err)
	}
}

func TestResponseBodyLimit(t *testing.T) {
	if _, err := readResponseBody(strings.NewReader(strings.Repeat("x", maxResponseBytes+1))); err == nil {
		t.Fatal("oversized response accepted")
	}
}

func TestSignedRequestsDoNotFollowRedirects(t *testing.T) {
	original := httpClient
	copyClient := *httpClient
	calls := 0
	copyClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://example.com/other"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	httpClient = &copyClient
	t.Cleanup(func() { httpClient = original })
	if _, err := testClient(t).GetOwnedDevices(); err == nil {
		t.Fatal("redirect accepted")
	}
	if calls != 1 {
		t.Fatal("followed redirect with credentials")
	}
}
