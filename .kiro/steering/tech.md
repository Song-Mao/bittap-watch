
---

# RULES.md — AI 编码规范与工程约束（Go / WS 延迟套利验证器）

> 项目：Bittap vs OKX/Binance USDT 永续 lead-lag 机会验证器（验证阶段：**只采集行情 + 统计 + 影子成交**，**严禁真实下单**）。
> 部署：GCP Tokyo，单进程，IPv4，4 核。

---

## 1) 项目硬性范围约束（Hard Scope）

### 1.1 禁止真实交易（绝对禁止）

* **禁止**任何真实下单/撤单/改单/杠杆/资金划转/签名交易请求/私有 WS。
* 禁止出现：REST `POST /order`、私有 WS `login`、API Key 权限、签名模块、订单管理器、撮合器用于真实交易。
* 本项目只允许：

  * 公共行情 WS
  * 合约元数据 REST（启动时拉取匹配）
  * 本地统计与落盘

### 1.2 Leader-Follower 单边模型（必须遵守）

* Leader：OKX、Binance（两条链路**独立统计**）
* Follower：Bittap（paper execution 也只用 Bittap 的可成交价）
* 两条链路（OKX->Bittap / Binance->Bittap）不共享 position/状态机。

---

## 2) Go 工程标准（代码结构、可读性、可维护性）

### 2.1 Go 版本与依赖

* Go >= 1.22（建议 1.23+）
* 禁止引入“大而全”的框架；依赖尽量少，必须写在 `go.mod` 且可复现。
* 所有第三方依赖必须：

  * 有明确用途
  * 不引入交易能力/下单 SDK

### 2.2 目录结构（必须）

```
cmd/validator/main.go

internal/
  config/          # config.yaml 解析 + 默认值 + 校验
  metadata/        # 合约元数据 HTTP client + 映射(SymbolMap)
  exchange/
    okx/           # WS client + subscribe + parser
    binance/
    bittap/
  core/
    model/         # BookEvent, Quote, Trade, enums
    store/         # 最新订单簿缓存（单写者）
    signal/        # 价差计算 + 过滤器
    paper/         # shadow execution 状态机 + TP/SL/timeout
  stats/
    ev/            # rolling EV 计算
    latency/       # lead-lag 分布/分位数
  output/
    jsonl/         # 异步 writer
    csv/
  util/
    backoff/
    timeutil/
    fastparse/
```

### 2.3 命名与风格（必须）

* 遵循 Go 官方风格：`gofmt`、`go vet` 必过。
* 命名：

  * 包名全小写、短且语义明确：`metadata`, `latency`, `paper`
  * 导出类型/函数用 PascalCase，非导出用 camelCase
  * 不要用 `Mgr/Util/Common` 这种含混命名
* 文件命名：

  * `*_test.go` 用于测试
  * 交易所适配器：`client.go`, `parser.go`, `types.go`

### 2.4 错误处理（必须）

* 错误必须带上下文：`fmt.Errorf("parse okx books5: %w", err)`
* 禁止吞错：禁止空 `catch` / ignore
* 对解析错误必须：

  * `parse_error_count++`
  * 采样记录原始消息（限速/采样，防止刷盘）

### 2.5 Context 与取消（必须）

* 所有 goroutine 必须可被 `context.Context` 取消退出
* 主进程退出时必须优雅关闭：

  * WS 连接
  * writer flush
  * metrics 最后一轮输出

---

## 3) 并发与性能规范（Low Latency / 可解释）

### 3.1 Goroutine/Channel 架构（必须）

* 每交易所一个 WS 连接 goroutine（读循环）
* 解析与归一化不得阻塞读循环（可同 goroutine 内做，但必须轻量；落盘/统计绝对不允许在读循环中做）
* 聚合器（signal engine）**单 goroutine**：唯一写入 book store + 状态机（避免锁与竞态）
* 输出写盘（JSONL/CSV）必须异步：writer goroutine + buffered channel

### 3.2 禁止共享可变状态（必须）

* Adapter 不得写全局 store
* store 只允许单写者（聚合器），读侧若需要快照必须通过消息或 copy（验证阶段优先单写单读）

### 3.3 分配与 GC 控制（必须）

* WS 消息 buffer、临时对象必须用 `sync.Pool` 复用（尤其是 `[]byte`）
* 解析路径禁止 `fmt.Sprintf`、禁止频繁 `map[string]any`
* 价格/数量字符串转换必须用 `strconv.ParseFloat`（起步）或封装 `fastparse`（后续替换）

### 3.4 日志性能（必须）

* 默认 `info`，`debug` 必须可配置且默认关闭
* 禁止在热路径打印每条行情日志（会毁掉延迟统计）

---

## 4) 数据契约与标准化（BookEvent / 时间戳 / Symbol 映射）

### 4.1 统一结构（必须实现）

必须有统一 `BookEvent`（或等价）：

* `Exchange`：`okx|binance|bittap`
* `SymbolCanon`：`BTCUSDT`
* `BestBidPx, BestBidQty`
* `BestAskPx, BestAskQty`
* `ArrivedAtUnixNs`：本机收到时间（延迟统计主基准）
* `ExchTsUnixMs`：OKX(ts)/Binance(E)/Bittap=0
* `Seq`：OKX(seqId)/Bittap(lastUpdateId)/Binance=0

### 4.2 时间戳规则（必须）

* lead-lag 主定义：`ArrivedAt`（本机）
* `ExchTs` 仅用于辅助/质量检查（Bittap 无 server ts）

### 4.3 Symbol 映射（必须）

* 启动时调用三家合约元数据 API（由你提供）
* 用户输入如 `BTC-USDT`：

  * 匹配 OKX instId / Binance symbol / Bittap symbol+tick
* 内部统一 `SymbolCanon` 用于 join
* 禁止在代码里硬编码订阅 symbol（只能来自映射结果 + config）

---

## 5) 信号/过滤器/影子成交/EV（验证闭环）

### 5.1 入场条件（必须）

对每条 leader 链路独立计算（Leader=A，Follower=B=Bittap）：

* 多头：`A_bid - B_ask > θ_entry`
* 空头：`B_bid - A_ask > θ_entry`

### 5.2 过滤器（必须、配置化）

* 持续时间过滤：`persist_ms`（100–300ms）
* 波动率过滤：1min realized vol > 阈值跳过（可关）
* SL 冷却：`cooldown_ms`（3–5s）

### 5.3 影子成交（必须，最小版）

* 默认：`taker_bbo`（保守）
* 允许 `slippage_bps` 配置
* 必须扣手续费（含返佣后的 effective fee）

### 5.4 TP/SL/超时退出（必须）

* TP：价差收敛（比例/阈值模型二选一，配置化）
* SL：价差发散
* 超时：`max_hold` 必须存在（防不收敛尾部风险）

### 5.5 EV 模块（必须）

* 滚动估计：`p, R, L, f`
* 输出：`EV、p_required、p_observed、RejectedByEV`

---

## 6) WebSocket 稳定性与重连（必须）

### 6.1 指数退避重连（必须）

* base 1s，max 30s，jitter ±20%
* 任意断线/读错误/心跳超时必须自动重连

### 6.2 心跳（必须）

* OKX：应用层 ping/pong（配置间隔）
* Binance：协议层 ping/pong（设置读超时）
* Bittap：JSON PING（配置间隔）

### 6.3 质量指标（必须输出）

* `last_message_age_ms`
* `reconnect_count`
* `parse_error_count`
* `updates_per_sec`（按交易所/按 symbol）

---

## 7) 输出规范（必须落盘、可复现）

### 7.1 文件（至少三类）

* `signals.jsonl`
* `paper_trades.jsonl`
* `metrics.jsonl`

### 7.2 paper_trades 必含字段

* `leader`（okx/binance）
* `symbol_canon`
* `side`（long/short）
* `t_entry_ns`, `t_exit_ns`
* `entry_px`, `exit_px`
* `gross_pnl_bps`, `fee_bps`, `net_pnl_bps`
* `exit_reason`（tp/sl/timeout）
* `ev_snapshot`（可选）

---

## 8) 配置文件规范（config.yaml）

### 8.1 必须支持

* symbols（用户输入）
* 元数据 API 地址
* ws 地址与订阅参数（从映射层拼装）
* fees（maker/taker + rebate_rate）
* strategy（θ_entry、persist、tp/sl、max_hold、vol filter、cooldown）
* output 目录与开关

### 8.2 配置校验（必须）

启动时校验：

* fee 范围合法（0~1）
* θ_entry、persist、max_hold 合理
* symbols 非空且映射成功，否则 fail fast

---

## 9) Golang 代码规范细则（强制执行）

### 9.1 格式化与静态检查（必须）

* `gofmt` 必须通过（CI 或 pre-commit）
* `go vet` 必须通过
* `staticcheck`（建议强制）
* `golangci-lint`（建议强制，至少启用：govet、staticcheck、errcheck、ineffassign、gofmt）

### 9.2 文档与注释（必须）

* 导出类型/函数必须写注释（GoDoc 风格）
* 核心状态机与关键公式必须注释清晰（ΔP、TP/SL、fee、EV）
* 每个交易所 parser 必须写字段映射注释（来自响应字段）

### 9.3 结构体设计（必须）

* 热路径结构体字段顺序按对齐（减少 padding）
* 避免在热路径使用 `map[string]interface{}` 作为中间结构
* 解析 JSON 时尽量用结构体（字段 string->float64 由专用转换函数处理）

### 9.4 日志规范（必须）

* 使用结构化 logger（zap/zerolog 二选一）
* 日志必须带关键维度：

  * exchange、leader、symbol、conn_id、reconnect_attempt
* 限速：解析错误原始消息日志必须采样（例如 1/100 或每分钟最多 N 条）

### 9.5 测试规范（必须有）

* 至少提供：

  * symbol 映射测试（元数据 mock）
  * parser 测试（每交易所 3 条样例消息）
  * paper 状态机测试（TP/SL/timeout）
  * EV 计算测试（边界值/手算对齐）
* 使用 `testing` + table-driven tests

### 9.6 可观察性（建议强烈）

* 暴露 `/metrics`（Prometheus）或至少 JSONL metrics
* 记录每交易所 update rate 与消息间隔分布（帮助解释 lag）

---

## 10) 交付验收清单（AI 输出代码必须满足）

* `go run cmd/validator/main.go --config config.yaml` 可启动
* 启动会拉取元数据并完成 symbol 映射，随后订阅三家 WS
* 自动重连正常、心跳正常
* 10 秒内产出 metrics 输出
* 出现触发后能产出 signals/paper_trades 输出
* OKX->Bittap 与 Binance->Bittap 指标分离
* 仓库中 **0** 真实下单相关代码

---

## 11) AI 编码时禁止事项（DON’T）

* 不得添加真实下单/私有通道/签名/密钥读写
* 不得硬编码 symbol、channel（必须来自元数据映射）
* 不得在 WS 读循环里落盘或做 heavy 统计
* 不得合并两条 leader 链路的状态机
* 不得只在内存里统计（必须落盘可复现）

---
