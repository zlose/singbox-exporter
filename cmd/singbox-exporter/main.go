package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/zlose/singbox-exporter/internal/config"
	"github.com/zlose/singbox-exporter/internal/exporter"
	"github.com/zlose/singbox-exporter/internal/prober"
	"github.com/zlose/singbox-exporter/internal/singbox"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("exporter stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	store := prober.NewStore()
	client := singbox.NewClient(cfg.SingboxURL, cfg.Secret, cfg.RequestTimeout, cfg.ProbeConcurrency)
	probe := prober.New(cfg, client, store, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go probe.Run(ctx)

	registry := prometheus.NewRegistry()
	registry.MustRegister(exporter.NewCollector(store))

	mux := http.NewServeMux()
	mux.Handle(cfg.MetricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc(cfg.HealthPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("starting singbox-exporter",
		"listen_address", cfg.ListenAddress,
		"metrics_path", cfg.MetricsPath,
		"health_path", cfg.HealthPath,
		"singbox_url", cfg.SingboxURL,
		"probe_url", cfg.ProbeURL,
		"scrape_interval", cfg.ScrapeInterval,
		"request_timeout", cfg.RequestTimeout,
		"probe_concurrency", cfg.ProbeConcurrency,
	)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("HTTP server failed: %w", err)
	}
}
