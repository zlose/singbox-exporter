# singbox-exporter 开发文档

## 1. 项目目标

`singbox-exporter` 是一个独立的 Prometheus exporter，用于从 sing-box 的 Clash API 采集代理节点状态，并暴露 Prometheus 格式指标。

第一阶段只关注两个核心能力：

- 代理节点可用性
- 代理节点延迟

不采集流量、连接、规则、内存等指标。

## 2. 接入方式

exporter 通过 sing-box 的 `experimental.clash_api` 接入。

sing-box 示例配置：

```json
{
  "experimental": {
    "clash_api": {
      "external_controller": "127.0.0.1:9090",
      "secret": "optional-secret"
    }
  }
}
```

exporter 访问以下 API：

```text
GET /proxies
GET /proxies/{name}/delay?timeout=5000&url=https://www.gstatic.com/generate_204
```

认证规则：

- 如果配置了 `--secret`，请求头增加 `Authorization: Bearer <secret>`。
- 如果未配置 `--secret`，不发送认证头。
- `secret` 不提供默认值。

## 3. 功能范围

### 3.1 第一阶段必须实现

- 暴露 `/metrics`。
- 暴露 `/healthz`。
- 从 `/proxies` 获取代理列表。
- 过滤出实际代理节点。
- 周期性调用 `/proxies/{name}/delay`。
- 输出节点可用性指标。
- 输出节点延迟指标。
- 支持命令行参数配置。
- 支持 include/exclude 正则过滤节点。
- 支持 sing-box API secret。
- sing-box API 异常时 exporter 不崩溃。

### 3.2 第一阶段不实现

- 流量统计。
- 连接数统计。
- 规则统计。
- Grafana Dashboard。
- Docker 镜像。
- Kubernetes manifest。
- 多 sing-box 实例采集。
- 动态重载配置。

## 4. 推荐技术栈

语言：Go

主要依赖：

```text
github.com/prometheus/client_golang/prometheus
github.com/prometheus/client_golang/prometheus/promhttp
```

可选依赖：

```text
github.com/spf13/pflag
```

也可以直接使用 Go 标准库 `flag`，第一版建议保持简单。

## 5. 项目结构

```text
singbox-exporter/
  cmd/
    singbox-exporter/
      main.go
  internal/
    config/
      config.go
    singbox/
      client.go
      types.go
    prober/
      prober.go
    exporter/
      collector.go
  go.mod
  README.md
  DEVELOPMENT.md
```

目录职责：

```text
cmd/singbox-exporter
  程序入口，负责读取配置、初始化 client、prober、collector 和 HTTP server。

internal/config
  命令行参数、默认值、配置校验。

internal/singbox
  sing-box Clash API client 和响应结构体。

internal/prober
  后台探测逻辑，定时拉取代理列表并探测延迟。

internal/exporter
  Prometheus collector，将 prober 缓存状态导出为指标。
```

## 6. 命令行参数

默认参数：

```text
--listen-address=:9112
--metrics-path=/metrics
--health-path=/healthz
--singbox-url=http://127.0.0.1:9090
--secret=
--probe-url=https://www.gstatic.com/generate_204
--probe-timeout=5000ms
--scrape-interval=30s
--request-timeout=10s
--probe-concurrency=64
--include-proxy-regex=
--exclude-proxy-regex=
```

参数说明：

```text
--listen-address
  exporter HTTP 服务监听地址。

--metrics-path
  Prometheus metrics 路径。

--health-path
  健康检查路径。

--singbox-url
  sing-box Clash API 地址。

--secret
  sing-box Clash API secret。为空时不发送 Authorization 头。

--probe-url
  用于 sing-box delay API 探测的目标 URL。

--probe-timeout
  单个节点 delay 探测超时时间，会传给 sing-box delay API。

--scrape-interval
  exporter 后台探测间隔。

--request-timeout
  exporter 调用 sing-box API 的 HTTP 请求超时时间。

--probe-concurrency
  节点 delay 探测最大并发数。实现上必须为节点探测启动 goroutine，并通过 semaphore 控制同时执行的请求上限。

--include-proxy-regex
  只采集匹配该正则的代理节点。为空表示不过滤。

--exclude-proxy-regex
  排除匹配该正则的代理节点。为空表示不过滤。
```

## 7. 指标设计

### 7.1 全局 API 状态

```text
singbox_api_up
```

含义：

```text
1 = exporter 最近一次成功访问 sing-box API
0 = exporter 最近一次访问 sing-box API 失败
```

### 7.2 节点可用性

```text
singbox_proxy_up{proxy="香港-01",type="Shadowsocks"}
```

含义：

```text
1 = 最近一次 delay 探测成功
0 = 最近一次 delay 探测失败
```

### 7.3 节点延迟

```text
singbox_proxy_delay_milliseconds{proxy="香港-01",type="Shadowsocks"}
```

含义：

```text
最近一次成功 delay 探测返回的延迟，单位毫秒
```

失败时策略：

- 不更新 delay 值。
- `singbox_proxy_up` 设置为 `0`。

### 7.4 单节点探测成功状态

```text
singbox_proxy_scrape_success{proxy="香港-01",type="Shadowsocks"}
```

含义：

```text
1 = exporter 成功完成该节点探测
0 = exporter 探测该节点失败
```

### 7.5 单节点探测耗时

```text
singbox_proxy_scrape_duration_seconds{proxy="香港-01",type="Shadowsocks"}
```

含义：

```text
exporter 调用 sing-box delay API 的耗时，单位秒
```

## 8. Label 规范

允许的 label：

```text
proxy
type
```

不建议加入以下 label：

```text
url
error
reason
status_code
```

原因：

- 错误文本可能导致高基数。
- URL 可能变化。
- Prometheus label 应保持稳定。
- 详细错误应进入日志，而不是指标 label。

## 9. 代理过滤规则

sing-box `/proxies` 返回内容通常包含实际代理节点和代理组。

第一阶段默认跳过以下类型：

```text
Selector
URLTest
Fallback
Direct
Reject
```

保留实际代理节点，例如：

```text
Shadowsocks
VMess
VLESS
Trojan
Hysteria
Hysteria2
TUIC
WireGuard
SSH
HTTP
SOCKS
```

实际实现中应避免强依赖完整类型列表。

推荐策略：

1. 如果 type 是明显的代理组类型，跳过。
2. 如果配置了 include regex，不匹配则跳过。
3. 如果配置了 exclude regex，匹配则跳过。
4. 其余节点进入探测队列。

## 10. 后台探测模型

不建议在 Prometheus 请求 `/metrics` 时实时探测所有节点。

推荐模型：

```text
启动 exporter
  -> 启动后台 prober
  -> 每 scrape-interval 执行一轮采集
  -> 为本轮采集创建 context.WithTimeout
  -> 获取 /proxies
  -> 过滤节点
  -> 使用 goroutine 高并发执行 delay 探测
  -> 通过 channel 聚合探测结果
  -> 批量更新内存缓存
  -> /metrics 只读取缓存
```

优势：

- `/metrics` 响应稳定。
- 避免 Prometheus scrape 超时。
- 避免多个 Prometheus 重复触发探测。
- 可以统一控制探测并发和总超时。
- sing-box API 异常不会阻塞 metrics 暴露。

每一轮采集必须有清晰的生命周期：

- 本轮开始时创建 `context.Context`。
- `/proxies` 请求和所有 delay 请求都继承该 context。
- 本轮超时或 exporter 停止时，所有未完成的 goroutine 必须尽快退出。
- 不允许出现泄漏 goroutine、阻塞 channel send、无限等待结果的情况。

## 11. 并发模型

第一阶段必须用 goroutine 做高并发节点探测，但并发必须可控，不能为无限制并发。

推荐实现：每个节点一个 goroutine，使用 semaphore 限制同时执行的 HTTP 请求数。

并发控制参数：

```text
--probe-concurrency=64
```

默认值选择 `64`，原因：

- sing-box 节点数量通常可能达到几十到几百个。
- delay API 是 I/O 密集型，请求等待时间远大于 CPU 计算时间。
- 64 个并发可以明显缩短整轮采集耗时。
- 仍然能避免一次性把 sing-box Clash API 打满。

如果运行环境性能较低，或 sing-box API 和 exporter 不在同一台机器上，可以调低到 `10` 或 `20`。

核心要求：

- 每个节点探测独立 goroutine。
- 使用 `semaphore := make(chan struct{}, concurrency)` 控制最大并发。
- 使用 `sync.WaitGroup` 等待本轮所有 goroutine 完成。
- 使用带缓冲的 `results` channel，容量为本轮待探测节点数，避免 goroutine 因结果写入阻塞。
- 使用 context 取消超时探测。
- collector 不直接发起探测，只读取缓存。

伪代码：

```go
ctx, cancel := context.WithTimeout(parentCtx, roundTimeout)
defer cancel()

sem := make(chan struct{}, concurrency)
results := make(chan ProbeResult, len(proxies))

var wg sync.WaitGroup
for _, proxy := range proxies {
    proxy := proxy
    wg.Add(1)

    go func() {
        defer wg.Done()

        select {
        case sem <- struct{}{}:
            defer func() { <-sem }()
        case <-ctx.Done():
            results <- ProbeResult{
                Name:          proxy.Name,
                Type:          proxy.Type,
                Up:            false,
                ScrapeSuccess: false,
                Error:         ctx.Err(),
            }
            return
        }

        started := time.Now()
        result := probeDelay(ctx, proxy)
        result.ScrapeDuration = time.Since(started)

        select {
        case results <- result:
        case <-ctx.Done():
        }
    }()
}

wg.Wait()
close(results)

batch := make([]ProbeResult, 0, len(proxies))
for result := range results {
    batch = append(batch, result)
}

cache.ReplaceBatch(batch)
```

### 11.1 HTTP 连接池

HTTP client 必须复用连接，避免高并发时频繁建连。

建议配置：

```go
transport := &http.Transport{
    MaxIdleConns:        256,
    MaxIdleConnsPerHost: 256,
    MaxConnsPerHost:     0,
    IdleConnTimeout:     90 * time.Second,
}

client := &http.Client{
    Transport: transport,
    Timeout:   requestTimeout,
}
```

说明：

- `MaxIdleConnsPerHost` 应大于或等于 `--probe-concurrency`。
- 如果后续发现 sing-box API 压力过大，可以设置 `MaxConnsPerHost` 等于 `--probe-concurrency`。
- 不要为每个请求创建新的 `http.Client`。

### 11.2 超时策略

建议区分三层超时：

- `--probe-timeout`：传给 sing-box delay API，由 sing-box 控制节点探测超时。
- `--request-timeout`：exporter HTTP client 单请求超时。
- `roundTimeout`：整轮采集最大耗时，建议为 `request-timeout * ceil(nodeCount / probeConcurrency) + 5s`，并设置上限。

如果实现复杂，第一版可以用固定整轮超时：

```text
roundTimeout = scrape-interval
```

要求：下一轮采集开始前，上一轮不能无限挂起。如果上一轮仍未结束，应跳过本轮或取消上一轮，第一版建议跳过本轮并记录 warn 日志。

### 11.3 结果聚合

所有 goroutine 只负责生成 `ProbeResult`，不直接逐条写 Prometheus 指标。

推荐结果结构：

```go
type ProbeResult struct {
    Name           string
    Type           string
    Up             bool
    DelayMillis    float64
    ScrapeSuccess  bool
    ScrapeDuration time.Duration
    Error          error
}
```

结果聚合要求：

- goroutine 将结果写入带缓冲 channel。
- 主 goroutine 关闭 results 后统一读取。
- 聚合完毕后批量更新缓存。
- 日志记录失败节点，但不要把错误文本写入 metrics label。

## 12. 缓存模型

prober 维护一份内存状态：

```go
type ProxyStatus struct {
    Name           string
    Type           string
    Up             bool
    DelayMillis    float64
    ScrapeSuccess  bool
    ScrapeDuration time.Duration
    LastScrapeTime time.Time
}
```

缓存要求：

- 读写并发安全。
- `/metrics` 只读缓存。
- 后台 prober 批量写缓存。
- 节点从 `/proxies` 消失后，应从缓存删除。
- 探测失败时保留上一次成功 delay，但 `up=false`。
- 缓存更新应尽量短时间持锁，不要在锁内执行 HTTP 请求、日志输出或复杂计算。
- 高并发探测结果先在局部 slice 中聚合，再一次性替换缓存快照。

推荐缓存结构：

```go
type Store struct {
    mu       sync.RWMutex
    apiUp    bool
    statuses map[string]ProxyStatus
}
```

推荐读写方式：

- `Snapshot()` 使用 `RLock` 复制当前状态后立即释放锁。
- `ReplaceBatch()` 使用 `Lock` 批量替换状态后立即释放锁。
- Prometheus collector 使用 `Snapshot()`，避免长时间持有读锁。

## 13. HTTP API Client

client 负责：

- 统一拼接 sing-box API URL。
- 设置 request timeout。
- 设置 Authorization header。
- JSON 解析。
- 返回结构化错误。

请求头：

```text
Authorization: Bearer <secret>
```

仅当 secret 非空时发送。

## 14. sing-box 响应结构

`GET /proxies` 响应大致结构：

```json
{
  "proxies": {
    "香港-01": {
      "type": "Shadowsocks",
      "all": [],
      "now": "",
      "history": []
    }
  }
}
```

第一阶段只需要：

```go
type ProxiesResponse struct {
    Proxies map[string]Proxy `json:"proxies"`
}

type Proxy struct {
    Type string `json:"type"`
}
```

`GET /proxies/{name}/delay` 响应：

```json
{
  "delay": 128
}
```

结构：

```go
type DelayResponse struct {
    Delay int `json:"delay"`
}
```

## 15. 错误处理策略

### 15.1 `/proxies` 失败

行为：

- `singbox_api_up = 0`。
- 不崩溃。
- 记录错误日志。
- 已知节点可标记为 `up=0`。
- 下一轮继续重试。

### 15.2 单节点 delay 失败

行为：

- 该节点 `singbox_proxy_up = 0`。
- 该节点 `singbox_proxy_scrape_success = 0`。
- 不影响其他节点。
- 记录 debug 或 warn 日志。

### 15.3 认证失败

HTTP 401 或 403：

- `singbox_api_up = 0`。
- 日志明确提示检查 `--secret`。
- 下一轮继续重试。

### 15.4 配置错误

启动时直接失败：

- `--singbox-url` 不是合法 URL。
- `--probe-url` 为空。
- `--scrape-interval <= 0`。
- `--probe-concurrency <= 0`。
- include/exclude regex 无法编译。

高并发相关配置校验：

- `--probe-concurrency` 必须大于 `0`。
- `--probe-concurrency` 不建议超过 `1024`，第一版可以超过时启动失败，避免错误配置导致瞬时创建过多 goroutine。
- `--request-timeout` 必须大于 `0`。
- `--scrape-interval` 应大于或等于 `--request-timeout`，否则高并发探测轮次容易重叠。

## 16. 日志设计

建议使用 Go 标准库 `log/slog`。

日志级别：

```text
info
  exporter 启动
  配置摘要
  HTTP 服务监听地址

warn
  单个节点探测失败
  sing-box API 临时不可用

error
  配置非法
  HTTP server 启动失败
```

不要在日志中打印 secret。

## 17. `/healthz`

第一阶段 `/healthz` 可以简单返回：

```text
200 OK
```

响应体：

```text
ok
```

注意：

- `/healthz` 表示 exporter 进程存活。
- 不代表 sing-box API 一定可用。
- sing-box API 状态通过 `singbox_api_up` 判断。

## 18. Prometheus 配置示例

```yaml
scrape_configs:
  - job_name: singbox
    static_configs:
      - targets:
          - 127.0.0.1:9112
```

## 19. 运行示例

无 secret：

```bash
singbox-exporter \
  --listen-address=:9112 \
  --singbox-url=http://127.0.0.1:9090
```

有 secret：

```bash
singbox-exporter \
  --listen-address=:9112 \
  --singbox-url=http://127.0.0.1:9090 \
  --secret='your-secret'
```

只采集部分节点：

```bash
singbox-exporter \
  --include-proxy-regex='香港|日本|新加坡'
```

排除部分节点：

```bash
singbox-exporter \
  --exclude-proxy-regex='备用|测试'
```

## 20. 开发顺序

建议按以下顺序实现：

1. 初始化 Go module。
2. 实现 config 解析和校验。
3. 实现 sing-box client。
4. 实现 `/proxies` 获取逻辑。
5. 实现 delay API 调用。
6. 实现代理过滤逻辑。
7. 实现 goroutine 高并发探测和 semaphore 限流。
8. 实现 prober 后台循环、context 取消和轮次超时。
9. 实现批量结果聚合和内存缓存。
10. 实现 Prometheus collector。
11. 实现 `/metrics`。
12. 实现 `/healthz`。
13. 增加 README 使用说明。
14. 增加基本单元测试。
15. 本地联调 sing-box。

## 21. 测试计划

### 21.1 单元测试

测试内容：

- 参数默认值。
- 参数校验。
- secret header 是否正确设置。
- `/proxies` 响应解析。
- delay 响应解析。
- include regex 过滤。
- exclude regex 过滤。
- 代理组类型过滤。
- 探测失败时状态更新。
- `--probe-concurrency` 边界值校验。
- 高并发探测不会超过配置的最大并发。
- context 取消后探测 goroutine 能退出。
- 批量缓存更新不会在 HTTP 请求期间持锁。

### 21.2 集成测试

使用 `httptest.Server` 模拟 sing-box Clash API。

场景：

- `/proxies` 成功。
- `/proxies` 返回 401。
- `/proxies` 返回非法 JSON。
- 单个 delay 成功。
- 单个 delay 超时。
- 部分节点失败，部分节点成功。
- 100 个以上节点并发探测时，整轮耗时符合并发预期。
- sing-box API 响应变慢时，上一轮不会无限阻塞下一轮。
- Prometheus 高频 scrape 时不会触发额外 delay 请求。

### 21.3 手动验证

启动 sing-box 后运行：

```bash
singbox-exporter \
  --singbox-url=http://127.0.0.1:9090
```

检查：

```bash
curl http://127.0.0.1:9112/healthz
curl http://127.0.0.1:9112/metrics
```

预期看到：

```text
singbox_api_up 1
singbox_proxy_up{proxy="...",type="..."} 1
singbox_proxy_delay_milliseconds{proxy="...",type="..."} ...
```

## 22. 第一阶段验收标准

第一阶段完成后应满足：

- exporter 可以作为单独二进制启动。
- 默认监听 `:9112`。
- 默认连接 `http://127.0.0.1:9090`。
- 支持空 secret。
- 支持非空 secret。
- `/healthz` 可用。
- `/metrics` 可用。
- 能输出代理节点 `up`。
- 能输出代理节点 delay。
- 使用 goroutine 并发探测节点，且并发数受 `--probe-concurrency` 控制。
- 高并发探测期间 `/metrics` 仍然只读缓存并快速返回。
- sing-box API 不可用时 exporter 不崩溃。
- Prometheus 可以正常 scrape。
- 节点过滤参数可用。

## 23. 后续扩展方向

第二阶段可考虑：

- Dockerfile。
- systemd unit 示例。
- Grafana Dashboard。
- 多 sing-box 实例采集。
- 流量指标。
- 连接数指标。
- 代理组当前选择指标。
- exporter 自身 build info。
- 配置文件支持。
- 动态 reload。
