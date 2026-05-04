package config

import "testing"

func TestDefaultValues(t *testing.T) {
	cfg := Default()
	if cfg.ListenAddress != ":9112" {
		t.Fatalf("unexpected listen address: %q", cfg.ListenAddress)
	}
	if cfg.MetricsPath != "/metrics" || cfg.HealthPath != "/healthz" {
		t.Fatalf("unexpected path defaults: %+v", cfg)
	}
	if cfg.SingboxURL != "http://127.0.0.1:9090" {
		t.Fatalf("unexpected singbox url: %q", cfg.SingboxURL)
	}
	if cfg.Secret != "" {
		t.Fatalf("secret should default to empty, got %q", cfg.Secret)
	}
	if cfg.ProbeURL != "https://www.gstatic.com/generate_204" {
		t.Fatalf("unexpected probe url: %q", cfg.ProbeURL)
	}
	if cfg.ProbeConcurrency != 64 {
		t.Fatalf("unexpected probe concurrency: %d", cfg.ProbeConcurrency)
	}
}

func TestParseAndValidate(t *testing.T) {
	cfg, err := Parse([]string{"--secret=s3cr3t", "--include-proxy-regex=^hk-", "--exclude-proxy-regex=test$"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.Secret != "s3cr3t" {
		t.Fatalf("unexpected secret: %q", cfg.Secret)
	}
	if cfg.IncludeProxyPattern == nil || !cfg.IncludeProxyPattern.MatchString("hk-01") {
		t.Fatalf("include regex not compiled")
	}
	if cfg.ExcludeProxyPattern == nil || !cfg.ExcludeProxyPattern.MatchString("foo-test") {
		t.Fatalf("exclude regex not compiled")
	}
}

func TestValidateRejectsBadValues(t *testing.T) {
	cases := []Config{
		{MetricsPath: "metrics", HealthPath: "/healthz", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "https://example.com", ProbeTimeout: 1, ScrapeInterval: 1, RequestTimeout: 1, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "healthz", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "https://example.com", ProbeTimeout: 1, ScrapeInterval: 1, RequestTimeout: 1, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "/metrics", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "https://example.com", ProbeTimeout: 1, ScrapeInterval: 1, RequestTimeout: 1, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "/healthz", SingboxURL: "ftp://bad", ProbeURL: "https://example.com", ProbeTimeout: 1, ScrapeInterval: 1, RequestTimeout: 1, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "/healthz", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "", ProbeTimeout: 1, ScrapeInterval: 1, RequestTimeout: 1, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "/healthz", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "https://example.com", ProbeTimeout: 0, ScrapeInterval: 1, RequestTimeout: 1, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "/healthz", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "https://example.com", ProbeTimeout: 1, ScrapeInterval: 0, RequestTimeout: 1, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "/healthz", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "https://example.com", ProbeTimeout: 1, ScrapeInterval: 1, RequestTimeout: 0, ProbeConcurrency: 1},
		{MetricsPath: "/metrics", HealthPath: "/healthz", SingboxURL: "http://127.0.0.1:9090", ProbeURL: "https://example.com", ProbeTimeout: 1, ScrapeInterval: 1, RequestTimeout: 2, ProbeConcurrency: 0},
	}
	for i, cfg := range cases {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("case %d expected error", i)
		}
	}
}
