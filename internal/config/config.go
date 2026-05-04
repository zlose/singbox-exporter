package config

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"regexp"
	"time"
)

const MaxProbeConcurrency = 1024

type Config struct {
	ListenAddress     string
	MetricsPath       string
	HealthPath        string
	SingboxURL        string
	Secret            string
	ProbeURL          string
	ProbeTimeout      time.Duration
	ScrapeInterval    time.Duration
	RequestTimeout    time.Duration
	ProbeConcurrency  int
	IncludeProxyRegex string
	ExcludeProxyRegex string

	IncludeProxyPattern *regexp.Regexp
	ExcludeProxyPattern *regexp.Regexp
}

func Default() Config {
	return Config{
		ListenAddress:    ":9112",
		MetricsPath:      "/metrics",
		HealthPath:       "/healthz",
		SingboxURL:       "http://127.0.0.1:9090",
		ProbeURL:         "https://www.gstatic.com/generate_204",
		ProbeTimeout:     5 * time.Second,
		ScrapeInterval:   30 * time.Second,
		RequestTimeout:   10 * time.Second,
		ProbeConcurrency: 64,
	}
}

func Parse(args []string) (Config, error) {
	cfg := Default()
	fs := flag.NewFlagSet("singbox-exporter", flag.ContinueOnError)
	fs.StringVar(&cfg.ListenAddress, "listen-address", cfg.ListenAddress, "exporter HTTP service listen address")
	fs.StringVar(&cfg.MetricsPath, "metrics-path", cfg.MetricsPath, "metrics endpoint path")
	fs.StringVar(&cfg.HealthPath, "health-path", cfg.HealthPath, "health endpoint path")
	fs.StringVar(&cfg.SingboxURL, "singbox-url", cfg.SingboxURL, "sing-box Clash API base URL")
	fs.StringVar(&cfg.Secret, "secret", cfg.Secret, "sing-box Clash API secret")
	fs.StringVar(&cfg.ProbeURL, "probe-url", cfg.ProbeURL, "URL used by sing-box delay API")
	fs.DurationVar(&cfg.ProbeTimeout, "probe-timeout", cfg.ProbeTimeout, "single proxy delay timeout")
	fs.DurationVar(&cfg.ScrapeInterval, "scrape-interval", cfg.ScrapeInterval, "background scrape interval")
	fs.DurationVar(&cfg.RequestTimeout, "request-timeout", cfg.RequestTimeout, "sing-box API HTTP request timeout")
	fs.IntVar(&cfg.ProbeConcurrency, "probe-concurrency", cfg.ProbeConcurrency, "maximum concurrent delay probes")
	fs.StringVar(&cfg.IncludeProxyRegex, "include-proxy-regex", cfg.IncludeProxyRegex, "only collect proxies matching this regex")
	fs.StringVar(&cfg.ExcludeProxyRegex, "exclude-proxy-regex", cfg.ExcludeProxyRegex, "exclude proxies matching this regex")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func (c *Config) Validate() error {
	if c.ListenAddress == "" {
		return errors.New("listen-address must not be empty")
	}
	if c.MetricsPath == "" || c.MetricsPath[0] != '/' {
		return errors.New("metrics-path must start with /")
	}
	if c.HealthPath == "" || c.HealthPath[0] != '/' {
		return errors.New("health-path must start with /")
	}
	if c.MetricsPath == c.HealthPath {
		return errors.New("metrics-path and health-path must be different")
	}
	if _, err := parseHTTPURL(c.SingboxURL); err != nil {
		return fmt.Errorf("invalid singbox-url: %w", err)
	}
	if c.ProbeURL == "" {
		return errors.New("probe-url must not be empty")
	}
	if _, err := parseHTTPURL(c.ProbeURL); err != nil {
		return fmt.Errorf("invalid probe-url: %w", err)
	}
	if c.ProbeTimeout <= 0 {
		return errors.New("probe-timeout must be greater than 0")
	}
	if c.ScrapeInterval <= 0 {
		return errors.New("scrape-interval must be greater than 0")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("request-timeout must be greater than 0")
	}
	if c.ScrapeInterval < c.RequestTimeout {
		return errors.New("scrape-interval must be greater than or equal to request-timeout")
	}
	if c.ProbeConcurrency <= 0 || c.ProbeConcurrency > MaxProbeConcurrency {
		return fmt.Errorf("probe-concurrency must be between 1 and %d", MaxProbeConcurrency)
	}
	if c.IncludeProxyRegex != "" {
		re, err := regexp.Compile(c.IncludeProxyRegex)
		if err != nil {
			return fmt.Errorf("invalid include-proxy-regex: %w", err)
		}
		c.IncludeProxyPattern = re
	}
	if c.ExcludeProxyRegex != "" {
		re, err := regexp.Compile(c.ExcludeProxyRegex)
		if err != nil {
			return fmt.Errorf("invalid exclude-proxy-regex: %w", err)
		}
		c.ExcludeProxyPattern = re
	}
	return nil
}

func parseHTTPURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("scheme must be http or https")
	}
	if u.Host == "" {
		return nil, errors.New("host must not be empty")
	}
	return u, nil
}
