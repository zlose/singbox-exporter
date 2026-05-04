package exporter

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/zlose/singbox-exporter/internal/prober"
)

type Collector struct {
	store *prober.Store

	apiUp          *prometheus.Desc
	proxyUp        *prometheus.Desc
	proxyDelay     *prometheus.Desc
	scrapeSuccess  *prometheus.Desc
	scrapeDuration *prometheus.Desc
}

func NewCollector(store *prober.Store) *Collector {
	labels := []string{"proxy", "type"}
	return &Collector{
		store: store,
		apiUp: prometheus.NewDesc(
			"singbox_api_up",
			"Whether the last sing-box API request succeeded.",
			nil,
			nil,
		),
		proxyUp: prometheus.NewDesc(
			"singbox_proxy_up",
			"Whether the last delay probe for this proxy succeeded.",
			labels,
			nil,
		),
		proxyDelay: prometheus.NewDesc(
			"singbox_proxy_delay_milliseconds",
			"Last successful delay probe result in milliseconds.",
			labels,
			nil,
		),
		scrapeSuccess: prometheus.NewDesc(
			"singbox_proxy_scrape_success",
			"Whether the exporter completed the last probe for this proxy successfully.",
			labels,
			nil,
		),
		scrapeDuration: prometheus.NewDesc(
			"singbox_proxy_scrape_duration_seconds",
			"Duration of the last delay API request for this proxy in seconds.",
			labels,
			nil,
		),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.apiUp
	ch <- c.proxyUp
	ch <- c.proxyDelay
	ch <- c.scrapeSuccess
	ch <- c.scrapeDuration
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	snapshot := c.store.Snapshot()
	ch <- prometheus.MustNewConstMetric(c.apiUp, prometheus.GaugeValue, boolFloat(snapshot.APIUp))

	for _, status := range snapshot.Statuses {
		labels := []string{status.Name, status.Type}
		ch <- prometheus.MustNewConstMetric(c.proxyUp, prometheus.GaugeValue, boolFloat(status.Up), labels...)
		if status.DelayValid {
			ch <- prometheus.MustNewConstMetric(c.proxyDelay, prometheus.GaugeValue, status.DelayMillis, labels...)
		}
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, boolFloat(status.ScrapeSuccess), labels...)
		ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, status.ScrapeDuration.Seconds(), labels...)
	}
}

func boolFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
