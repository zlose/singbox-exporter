# singbox-exporter

独立的 Prometheus exporter，用于从 sing-box `experimental.clash_api` 采集代理节点可用性和延迟。

## 功能

- 暴露 `/metrics` 和 `/healthz`
- 从 Clash API `/proxies` 获取代理列表
- 周期性调用 `/proxies/{name}/delay`
- 支持 sing-box API secret
- 支持 include/exclude 正则过滤代理节点
- 后台并发探测，`/metrics` 只读取缓存

## 构建

```bash
go mod tidy
go build ./cmd/singbox-exporter
```

## 运行

无 secret：

```bash
./singbox-exporter \
  --listen-address=:9112 \
  --singbox-url=http://127.0.0.1:9090
```

有 secret：

```bash
./singbox-exporter \
  --singbox-url=http://127.0.0.1:9090 \
  --secret='your-secret'
```

过滤节点：

```bash
./singbox-exporter --include-proxy-regex='香港|日本|新加坡'
./singbox-exporter --exclude-proxy-regex='备用|测试'
```

## 参数

| 参数 | 默认值 |
| --- | --- |
| `--listen-address` | `:9112` |
| `--metrics-path` | `/metrics` |
| `--health-path` | `/healthz` |
| `--singbox-url` | `http://127.0.0.1:9090` |
| `--secret` | 空 |
| `--probe-url` | `https://www.gstatic.com/generate_204` |
| `--probe-timeout` | `5s` |
| `--scrape-interval` | `30s` |
| `--request-timeout` | `10s` |
| `--probe-concurrency` | `64` |
| `--include-proxy-regex` | 空 |
| `--exclude-proxy-regex` | 空 |

## 指标

```text
singbox_api_up
singbox_proxy_up{proxy="香港-01",type="Shadowsocks"}
singbox_proxy_delay_milliseconds{proxy="香港-01",type="Shadowsocks"}
singbox_proxy_scrape_success{proxy="香港-01",type="Shadowsocks"}
singbox_proxy_scrape_duration_seconds{proxy="香港-01",type="Shadowsocks"}
```

## Prometheus 配置示例

```yaml
scrape_configs:
  - job_name: singbox
    static_configs:
      - targets:
          - 127.0.0.1:9112
```

## 健康检查

```bash
curl http://127.0.0.1:9112/healthz
curl http://127.0.0.1:9112/metrics
```
