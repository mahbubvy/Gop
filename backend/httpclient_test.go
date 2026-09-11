package backend

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHTTPClientWithProxyInitializesTLSConfig(t *testing.T) {
	t.Setenv("GODEBUG", "http2client=0")
	for _, proxyURL := range []string{"", "http://127.0.0.1:8080"} {
		t.Run("proxy="+proxyURL, func(t *testing.T) {
			previous := http.DefaultTransport
			base := &http.Transport{}
			http.DefaultTransport = base
			t.Cleanup(func() { http.DefaultTransport = previous })

			client, err := NewHTTPClientWithProxy(proxyURL)
			if err != nil {
				t.Fatal(err)
			}
			transport := client.Transport.(*http.Transport)
			defer transport.CloseIdleConnections()
			if transport.TLSClientConfig == nil {
				t.Fatal("TLS configuration was not initialized")
			}
			// Preserve existing proxy behavior; this fix only handles nil configs.
			if got, want := transport.TLSClientConfig.InsecureSkipVerify, proxyURL != ""; got != want {
				t.Fatalf("InsecureSkipVerify = %v, want %v", got, want)
			}
			if base.TLSClientConfig != nil {
				t.Fatal("client construction changed the original TLS configuration")
			}
		})
	}
}

func TestNewHTTPClientWithProxyPreservesTLSConfig(t *testing.T) {
	previous := http.DefaultTransport
	base := &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         "example.test",
		InsecureSkipVerify: true,
	}}
	http.DefaultTransport = base
	t.Cleanup(func() { http.DefaultTransport = previous })

	client, err := NewHTTPClientWithProxy("")
	if err != nil {
		t.Fatal(err)
	}
	transport := client.Transport.(*http.Transport)
	defer transport.CloseIdleConnections()
	config := transport.TLSClientConfig
	if config == base.TLSClientConfig {
		t.Fatal("client shares the original TLS configuration")
	}
	if config.MinVersion != tls.VersionTLS12 || config.ServerName != "example.test" {
		t.Fatal("existing TLS settings were lost")
	}
	if config.InsecureSkipVerify || !base.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("certificate verification was not enabled independently of the original config")
	}
}

func TestNewHTTPClientWithProxyNegotiatesProtocols(t *testing.T) {
	for _, test := range []struct {
		name      string
		goDebug   string
		wantMajor int
	}{
		{"HTTP1", "http2client=0", 1},
		{"HTTP2", "http2client=1", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GODEBUG", test.goDebug)
			previous := http.DefaultTransport
			// Use a fresh transport so protocol initialization runs under this setting.
			http.DefaultTransport = &http.Transport{ForceAttemptHTTP2: true}
			t.Cleanup(func() { http.DefaultTransport = previous })

			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.WriteString(w, "ok")
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()

			client, err := NewHTTPClientWithProxy("")
			if err != nil {
				t.Fatal(err)
			}
			client.Timeout = 5 * time.Second
			transport := client.Transport.(*http.Transport)
			defer transport.CloseIdleConnections()
			roots := x509.NewCertPool()
			roots.AddCert(server.Certificate())
			transport.TLSClientConfig.RootCAs = roots

			response, err := client.Get(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil || string(body) != "ok" {
				t.Fatalf("response body = %q, error = %v", body, err)
			}
			if response.ProtoMajor != test.wantMajor {
				t.Fatalf("protocol = %s, want HTTP/%d", response.Proto, test.wantMajor)
			}
		})
	}
}
