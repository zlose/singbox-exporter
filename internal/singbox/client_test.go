package singbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientAuthorizationHeader(t *testing.T) {
	t.Run("with secret", func(t *testing.T) {
		var gotAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Write([]byte(`{"proxies":{}}`))
		}))
		defer server.Close()

		client := NewClient(server.URL, "secret", time.Second, 1)
		if _, err := client.ListProxies(context.Background()); err != nil {
			t.Fatalf("ListProxies() error = %v", err)
		}
		if gotAuth != "Bearer secret" {
			t.Fatalf("unexpected auth header: %q", gotAuth)
		}
	})

	t.Run("without secret", func(t *testing.T) {
		var gotAuth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Write([]byte(`{"proxies":{}}`))
		}))
		defer server.Close()

		client := NewClient(server.URL, "", time.Second, 1)
		if _, err := client.ListProxies(context.Background()); err != nil {
			t.Fatalf("ListProxies() error = %v", err)
		}
		if gotAuth != "" {
			t.Fatalf("auth header should be omitted, got %q", gotAuth)
		}
	})
}

func TestClientParsesProxiesAndDelay(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/proxies":
			w.Write([]byte(`{"proxies":{"node-a":{"type":"Shadowsocks"},"group":{"type":"Selector"}}}`))
		case "/proxies/node-a/delay":
			if r.URL.Query().Get("timeout") != "5000" || r.URL.Query().Get("url") != "https://example.com" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"delay":123}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "", time.Second, 1)
	proxies, err := client.ListProxies(context.Background())
	if err != nil {
		t.Fatalf("ListProxies() error = %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("unexpected proxies len: %d", len(proxies))
	}
	delay, err := client.ProbeDelay(context.Background(), "node-a", "https://example.com", 5*time.Second)
	if err != nil {
		t.Fatalf("ProbeDelay() error = %v", err)
	}
	if delay != 123 {
		t.Fatalf("unexpected delay: %d", delay)
	}
}
