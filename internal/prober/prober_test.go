package prober

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zlose/singbox-exporter/internal/config"
	"github.com/zlose/singbox-exporter/internal/singbox"
)

type fakeClient struct {
	proxies  []singbox.Proxy
	delay    int
	err      error
	inflight int32
	maxSeen  int32
	block    chan struct{}
}

func (f *fakeClient) ListProxies(ctx context.Context) ([]singbox.Proxy, error) { return f.proxies, nil }

func (f *fakeClient) ProbeDelay(ctx context.Context, name, probeURL string, timeout time.Duration) (int, error) {
	cur := atomic.AddInt32(&f.inflight, 1)
	for {
		max := atomic.LoadInt32(&f.maxSeen)
		if cur <= max || atomic.CompareAndSwapInt32(&f.maxSeen, max, cur) {
			break
		}
	}
	defer atomic.AddInt32(&f.inflight, -1)
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	return f.delay, f.err
}

func TestFilterProxies(t *testing.T) {
	p := New(config.Default(), nil, NewStore(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.cfg.IncludeProxyPattern = nil
	p.cfg.ExcludeProxyPattern = nil
	filtered := p.filterProxies([]singbox.Proxy{{Name: "g", Type: "Selector"}, {Name: "a", Type: "Shadowsocks"}, {Name: "b", Type: "VLESS"}})
	if len(filtered) != 2 {
		t.Fatalf("unexpected len: %d", len(filtered))
	}
	// include/exclude filtering is validated through explicit compiled regexes.
	p.cfg.IncludeProxyPattern = nil
	p.cfg.ExcludeProxyPattern = nil
	p.cfg.IncludeProxyPattern = mustCompile(t, `^a$`)
	p.cfg.ExcludeProxyPattern = mustCompile(t, `^b$`)
	filtered = p.filterProxies([]singbox.Proxy{{Name: "a", Type: "Shadowsocks"}, {Name: "b", Type: "Shadowsocks"}, {Name: "c", Type: "Shadowsocks"}})
	if len(filtered) != 1 || filtered[0].Name != "a" {
		t.Fatalf("unexpected filtered proxies: %+v", filtered)
	}
}

func TestProbeConcurrencyLimit(t *testing.T) {
	store := NewStore()
	fc := &fakeClient{proxies: []singbox.Proxy{{Name: "a", Type: "Shadowsocks"}, {Name: "b", Type: "Shadowsocks"}, {Name: "c", Type: "Shadowsocks"}}, delay: 1, block: make(chan struct{})}
	cfg := config.Default()
	cfg.ProbeConcurrency = 2
	cfg.ScrapeInterval = 10 * time.Millisecond
	p := New(cfg, fc, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(50 * time.Millisecond); close(fc.block); cancel() }()
	p.scrape(ctx)
	if atomic.LoadInt32(&fc.maxSeen) > 2 {
		t.Fatalf("max concurrency %d exceeded limit", fc.maxSeen)
	}
}

func TestStoreKeepsLastDelayOnFailure(t *testing.T) {
	s := NewStore()
	s.ReplaceBatch([]ProbeResult{{Name: "a", Type: "Shadowsocks", Up: true, DelayMillis: 123, ScrapeSuccess: true}})
	s.ReplaceBatch([]ProbeResult{{Name: "a", Type: "Shadowsocks", Up: false, ScrapeSuccess: false, Error: errors.New("boom")}})
	snap := s.Snapshot()
	if len(snap.Statuses) != 1 || !snap.Statuses[0].DelayValid || snap.Statuses[0].DelayMillis != 123 {
		t.Fatalf("delay not preserved: %+v", snap.Statuses)
	}
}

func TestScrapeTimeoutKeepsExistingProxyAndLastDelay(t *testing.T) {
	store := NewStore()
	store.ReplaceBatch([]ProbeResult{{Name: "a", Type: "Shadowsocks", Up: true, DelayMillis: 123, ScrapeSuccess: true}})

	cfg := config.Default()
	cfg.ScrapeInterval = 10 * time.Millisecond
	p := New(cfg, &fakeClient{
		proxies: []singbox.Proxy{{Name: "a", Type: "Shadowsocks"}},
		block:   make(chan struct{}),
	}, store, slog.New(slog.NewTextHandler(io.Discard, nil)))

	p.scrape(context.Background())
	snap := store.Snapshot()
	if len(snap.Statuses) != 1 {
		t.Fatalf("expected proxy to remain in cache, got %+v", snap.Statuses)
	}
	status := snap.Statuses[0]
	if status.Up || status.ScrapeSuccess {
		t.Fatalf("timed-out proxy should be down: %+v", status)
	}
	if !status.DelayValid || status.DelayMillis != 123 {
		t.Fatalf("last delay not preserved: %+v", status)
	}
}

func mustCompile(t *testing.T, expr string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Fatalf("compile regex %q: %v", expr, err)
	}
	return re
}
