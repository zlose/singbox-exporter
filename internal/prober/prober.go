package prober

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/zlose/singbox-exporter/internal/config"
	"github.com/zlose/singbox-exporter/internal/singbox"
)

type ProbeResult struct {
	Name           string
	Type           string
	Up             bool
	DelayMillis    float64
	ScrapeSuccess  bool
	ScrapeDuration time.Duration
	Error          error
}

type Client interface {
	ListProxies(ctx context.Context) ([]singbox.Proxy, error)
	ProbeDelay(ctx context.Context, name, probeURL string, timeout time.Duration) (int, error)
}

type Prober struct {
	cfg    config.Config
	client Client
	store  *Store
	logger *slog.Logger
}

func New(cfg config.Config, client Client, store *Store, logger *slog.Logger) *Prober {
	return &Prober{cfg: cfg, client: client, store: store, logger: logger}
}

func (p *Prober) Run(ctx context.Context) {
	p.scrape(ctx)

	ticker := time.NewTicker(p.cfg.ScrapeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.scrape(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (p *Prober) scrape(parent context.Context) {
	roundCtx, cancel := context.WithTimeout(parent, p.cfg.ScrapeInterval)
	defer cancel()

	proxies, err := p.client.ListProxies(roundCtx)
	if err != nil {
		p.store.MarkAPIUnavailable()
		var statusErr singbox.StatusError
		if errors.As(err, &statusErr) && statusErr.IsAuthFailure() {
			p.logger.Warn("sing-box API authentication failed; check --secret", "error", err)
			return
		}
		p.logger.Warn("failed to list sing-box proxies", "error", err)
		return
	}

	proxies = p.filterProxies(proxies)
	results := make(chan ProbeResult, len(proxies))
	sem := make(chan struct{}, p.cfg.ProbeConcurrency)

	var wg sync.WaitGroup
	for _, proxy := range proxies {
		proxy := proxy
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-roundCtx.Done():
				results <- ProbeResult{Name: proxy.Name, Type: proxy.Type, Error: roundCtx.Err()}
				return
			}

			started := time.Now()
			delay, err := p.client.ProbeDelay(roundCtx, proxy.Name, p.cfg.ProbeURL, p.cfg.ProbeTimeout)
			result := ProbeResult{
				Name:           proxy.Name,
				Type:           proxy.Type,
				Up:             err == nil,
				ScrapeSuccess:  err == nil,
				ScrapeDuration: time.Since(started),
				Error:          err,
			}
			if err == nil {
				result.DelayMillis = float64(delay)
			} else {
				p.logger.Warn("failed to probe proxy delay", "proxy", proxy.Name, "type", proxy.Type, "error", err)
			}

			results <- result
		}()
	}

	wg.Wait()
	close(results)

	batch := make([]ProbeResult, 0, len(proxies))
	for result := range results {
		batch = append(batch, result)
	}
	p.store.ReplaceBatch(batch)
}

func (p *Prober) filterProxies(proxies []singbox.Proxy) []singbox.Proxy {
	filtered := make([]singbox.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if isProxyGroupType(proxy.Type) {
			continue
		}
		if p.cfg.IncludeProxyPattern != nil && !p.cfg.IncludeProxyPattern.MatchString(proxy.Name) {
			continue
		}
		if p.cfg.ExcludeProxyPattern != nil && p.cfg.ExcludeProxyPattern.MatchString(proxy.Name) {
			continue
		}
		filtered = append(filtered, proxy)
	}
	return filtered
}

func isProxyGroupType(proxyType string) bool {
	switch proxyType {
	case "Selector", "URLTest", "Fallback", "Direct", "Reject":
		return true
	default:
		return false
	}
}
