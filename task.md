
---

# Bittap vs OKX / Binance：USDT 永续合约延迟套利快速验证方案（Go / Tokyo GCP）

## 0. 目标与非目标

### 目标（验证阶段）

1. **测量 lead-lag**：OKX→Bittap、Binance→Bittap 的报价传播延迟分布（毫秒级）。
2. **测量可交易价差机会**：当 Leader 出现价格变化后，Bittap 在窗口内是否“跟随”，以及跟随需要多久。
3. **影子成交（Paper/Shadow Execution）**：不下单，但用订单簿推送模拟“以当时可成交价格进出”的 PnL，验证是否可能覆盖手续费与滑点。

### 非目标（本阶段不做）

* 不对冲、不跨所 Legging、不开真实订单。
* 不做多进程（单进程 4 核、IPv4）。

---

## 1. 总体架构（事件驱动，锁无关 Tick Pipeline）

### 1.1 数据流

* 三路行情输入（3 个 WS Client）：

  * OKX（Leader A1）
  * Binance（Leader A2）
  * Bittap（Follower B）
* 三个适配器将各自 WS 消息归一化为统一 `BookEvent`（Top5 深度即可）。
* 信号引擎分别对：

  * `OKX -> Bittap`
  * `Binance -> Bittap`
    独立统计与出“影子交易”。

> 入场逻辑来自 Leader-Follower 模型：只在 B 站操作；A 站仅提供信号。

### 1.2 核心 Goroutine / Channel 划分（建议）

* `wsReader(exchange)`：读 WS 原始消息 -> `rawCh`
* `parser(exchange)`：raw -> `BookEvent` -> `bookCh`
* `bookStore`：维护三家交易所最新 Top-of-Book（best bid/ask + depth5）
* `signalEngine(okx)` / `signalEngine(binance)`：读取最新 leader + follower book，计算 ΔP、过滤、生成 shadow trade
* `metricsReporter`：周期性输出统计（log + CSV/JSONL）

---

## 2. WebSocket 接入规范（按你给的项目整理）

### 2.1 OKX（public books5）

**连接地址**

* `wss://ws.okx.com:8443/ws/v5/public`

**连接 Header / Timeout**

* `User-Agent: cex-trade-golang/ws`
* `Origin: https://www.okx.com`
* `HandshakeTimeout: 10s`, `NetDialTimeout: 8s`

**订阅（books5）**

```json
{
  "id": "1",
  "op": "subscribe",
  "args": [{"channel": "books5", "instId": "BTC-USDT"}]
}
```



**返回数据关键字段**

* `data[0].asks / bids`
* `ts`（毫秒字符串）
* `seqId`（序列号）

**心跳**

* 客户端文本 `ping`，服务端文本 `pong`
* interval 25s、pongTimeout 10s；可记录 RTT 并超时强制重连

**OKX 输出归一化建议**

* `ExchTsUnixMs = parse(ts)`
* `Seq = seqId`
* `WsRttMs = lastRttMs`

---

### 2.2 Binance（USDT 合约 fstream depth5@100ms）

**连接地址**

* `wss://fstream.binance.com/ws`

**订阅**

```json
{
  "method": "SUBSCRIBE",
  "params": ["btcusdt@depth5@100ms", "ethusdt@depth5@100ms"],
  "id": 1
}
```



**推送字段（depthUpdate）**

* `e=depthUpdate`
* `E` 事件时间（ms）
* `s` Symbol（如 BTCUSDT）
* `b` bids、`a` asks（[[price, qty], ...] 字符串）

**心跳**

* 适配器不发自定义心跳，依赖 WS 协议层 ping/pong frame

**Binance 输出归一化建议**

* `ExchTsUnixMs = E`（你现在项目里“未填充”，验证阶段建议填上，便于 lead-lag 对齐）

---

### 2.3 Bittap（f_depth30@{symbol}_{tick}，现有文档是 30 档）

**连接地址**

* `wss://stream.bittap.com/endpoint?format=JSON`

**订阅格式**

```json
{
  "method": "SUBSCRIBE",
  "params": ["f_depth30@BTC-USDT-M_0.1"],
  "id": "fccmibd"
}
```

频道格式：`f_depth30@{交易对}_{档位}`

**心跳**

```json
{"id":"abc123","method":"PING"}
```

服务端返回 PONG（可能是 `{"result":"PONG"}` 或 `{"method":"PONG"}`）

**深度推送字段**

* `e = "f_depth30"`
* `s = "BTC-USDT-M"`
* `i = "0.1"`
* `lastUpdateId`（用于去重/同步）
* `bids` / `asks`（二维数组字符串）

> 注意：价格/数量是 string；bids 高->低，asks 低->高；PONG 可直接过滤。

---

## 3. 连接稳定性与重连策略（必须做，否则数据结论不可信）

### 3.1 通用断开码（参考）

1000/1001/1006/1008/1011 对应处理建议：自动重连、检查请求格式、延迟重连等。

### 3.2 退避重连（指数退避 + 抖动）

`ws.Config` 建议内置：

* `BackoffBase=1s`
* `BackoffMax=30s`
* `BackoffJitter=0.2 (±20%)`

### 3.3 交易所特化

* OKX：必须做 ping/pong RTT + pongTimeout，否则 Tokyo 机房偶发抖动会把延迟统计污染。
* Binance：协议层 ping/pong，重点是读超时与自动重连。
* Bittap：按 15–20s 发 PING。

---

## 4. 归一化数据结构（验证阶段最重要：时间戳与一致性）

### 4.1 统一 BookEvent（建议字段）

* `Exchange`：okx/binance/bittap
* `SymbolCanon`：统一成 `BTCUSDT`
* `BestBidPx, BestBidQty`
* `BestAskPx, BestAskQty`
* `LevelsBid[0..4], LevelsAsk[0..4]`（最多 5 档即可）
* `ArrivedAtUnixNs`：本机收到时刻（`time.Now()`）
* `ExchTsUnixMs`：交易所事件时间（OKX ts / Binance E；Bittap 无则置 0）
* `Seq`：OKX seqId；Bittap lastUpdateId；Binance 无则置 0
* `WsRttMs`：仅 OKX 有（或你自己加 socket-level 估算）

> 你当前 spec 已说明 OKX 有 `ts/seqId`，Binance 有 `E`，Bittap 没有服务器时间字段。

### 4.2 时间对齐策略（因为 Bittap 无 server ts）

* **Lead 事件时间**优先用：

  * OKX：`ExchTsUnixMs = ts`
  * Binance：`ExchTsUnixMs = E`
* **Follower（Bittap）**只能用 `ArrivedAt` 做近似时间轴。
* 因此 lead-lag 的主定义建议两套同时算：

  1. `Lag_arrival = B_arrived_at - A_arrived_at`（最直接，包含网络与解析开销）
  2. `Lag_event = B_arrived_at - A_exch_ts`（Leader 端更“真实”，但会混入你与交易所时钟偏差）

---

## 5. 信号与过滤器（只做验证也要严格，否则统计会虚高）

### 5.1 基础入场条件（Leader-Follower）

设 A=Leader，B=Bittap（Follower）：

* 多头：`ΔP_long = P^A_bid - P^B_ask > θ_entry`
* 空头：`ΔP_short = P^B_bid - P^A_ask > θ_entry`

并且：策略只在 B 上操作，不在 A 开仓。

### 5.2 入场过滤器（验证阶段必须统计“过滤前/后”）

* **价差持续时间**：ΔP 连续保持 > θ_entry 达到 100–300ms 才算有效信号
* **深度确认**：Leader 侧 top 档/前 5 档深度满足最小 USD（避免假突破）
* **波动率过滤**：若 1 分钟平均波动率 > 阈值（如 2%），跳过
* **冷却时间**：止损后 3–5s 不开新仓，防止震荡

---

## 6. 为什么我建议做“影子成交”（你问到的点）

仅做“有没有延迟”会得到很多**看起来能赚**的信号，但你真正关心的是：
**在 Bittap 上以可成交价格进出后，能否覆盖手续费 + 滑点**。

CODEX 已明确：延迟套利常见价差只有 0.1%–0.3%，手续费会把边际压成负值，必须严肃处理费用与返佣。

同时交易所可能有异常行为检测，极端会触发风控/影子禁令；验证阶段可以先用影子成交把“频率、节奏、可疑行为”量化出来（后续真单阶段再加随机抖动等）。

结论：**要验证“是否值得真单”，影子成交几乎是必选项。**

---

## 7. 影子成交模型（Paper Execution）——不下单，但模拟最坏可成交

### 7.1 成交价与滑点（建议保守）

* **入场**：默认用 Bittap 的 `best ask/bid` 做 taker 成交（最保守）
* **出场**：同样用 Bittap 的 `best ask/bid` 做 taker（再保守）
* 若你想更贴近未来 maker 退出，也可做两个版本：

  * `taker->taker`（下限）
  * `taker->maker`（更乐观，但要考虑挂单不成交风险）

> Maker 单不成交风险在 CODEX 里是核心风险之一；真单阶段一定要做超时回退等风控。

### 7.2 手续费（含返佣）

你给的费率：Bittap maker 0.02%，taker 0.06%，返佣 80%–90%。
验证阶段建议在 config 里显式写：

* `maker_fee = 0.0002`
* `taker_fee = 0.0006`
* `rebate_rate = 0.85`（可配 0.8~0.9）

并计算：

* `effective_fee = raw_fee * (1 - rebate_rate)`

### 7.3 EV 阻断（即使不下单也要统计 EV）

按 CODEX 的 EV：
[
E[Profit] = p(R - f) + (1 - p)(-L - f)
]
且盈亏平衡胜率：
[
p = (L + f)/(R + L)
]


落地方式（验证阶段）：

* 对每个信号生成一笔“影子交易”，记录其最终结果（TP/SL/超时），由此估计 `p、R、L`，再回算 EV。
* 若 EV < 0：标记为 `RejectedByEV`（后续真单直接阻断）。

---

## 8. 收敛/退出判定（用于影子成交结算）

### 8.1 退出模型（基于价差收敛）

CODEX 的建议：

* 以入场价差 `ΔP0` 为基准

  * TP：(|ΔP_t| \le (1-r_{tp})|ΔP_0|)
  * SL：(|ΔP_t| \ge (1+r_{sl})|ΔP_0|)

验证阶段你可以先用更直观的版本：

* TP：ΔP 回落到 `θ_exit` 或者回落到 `k * ΔP0`
* SL：ΔP 扩大到 `k_sl * ΔP0`

### 8.2 最大持仓时间（非常关键）

不收敛是延迟套利最大风险之一；建议强制最大持仓时间 10–25 秒（超时按 SL 或按市价退出计）。

---

## 9. 需要输出的统计指标（你跑 1 晚上就能得结论）

对 OKX->Bittap、Binance->Bittap 各自独立输出：

### 9.1 延迟类

* `Lag_arrival_ms` 分布：P50 / P90 / P99
* “Leader 变动 -> Bittap 首次同向变动”的跟随时间分布（ms）
* `OKX WsRttMs` 分布（用于解释网络波动）

### 9.2 机会类（过滤前/后）

* 原始 ΔP 超阈值次数
* 通过持续性过滤次数（100–300ms）
* 通过深度/波动率过滤次数

### 9.3 影子成交类（最重要）

* 影子交易笔数、胜率、平均 R/L、最大回撤（paper）
* 按手续费（含返佣）扣减后的净 PnL 分布
* EV（按滚动窗口估计 p/R/L/f）与 `RejectedByEV` 比例

---

## 10. config.yaml（你要“可填配置文件”，这里给模板）

> 你说启动时会提供三家交易所的合约元数据接口：启动时拉取并按你填写的 `BTC-USDT` 去匹配。此处先把“符号映射策略”写进 config。

```yaml
app:
  env: "prod"
  timezone: "Asia/Singapore"
  cpu_cores: 4
  ipv4_only: true

symbols:
  - user_input: "BTC-USDT"        # 你填 BTC-USDT
    canon: "BTCUSDT"              # 内部统一
    okx_instId: "BTC-USDT"        # OKX instId
    binance_native: "btcusdt"     # Binance stream native
    bittap_symbol: "BTC-USDT-M"   # Bittap symbol
    bittap_tick: "0.1"            # 订阅里的档位 i

metadata:
  enable: true
  okx_meta_url: "https://REPLACE_ME"
  binance_meta_url: "https://REPLACE_ME"
  bittap_meta_url: "https://REPLACE_ME"
  refresh_on_start: true

ws:
  common:
    handshake_timeout_ms: 10000
    dial_timeout_ms: 8000
    backoff_base_ms: 1000
    backoff_max_ms: 30000
    backoff_jitter: 0.2

  okx:
    url: "wss://ws.okx.com:8443/ws/v5/public"
    headers:
      Origin: "https://www.okx.com"
      User-Agent: "cex-trade-golang/ws"
    channel: "books5"
    heartbeat_interval_ms: 25000
    pong_timeout_ms: 10000

  binance:
    url: "wss://fstream.binance.com/ws"
    stream_suffix: "@depth5@100ms"

  bittap:
    url: "wss://stream.bittap.com/endpoint?format=JSON"
    channel_prefix: "f_depth30@"
    heartbeat_interval_ms: 18000

fees:
  # 仅用于 Bittap（Follower）的影子成交成本
  maker_fee: 0.0002
  taker_fee: 0.0006
  rebate_rate: 0.85          # 0.80~0.90 可调
  assume_entry: "taker"      # taker / maker
  assume_exit: "taker"       # taker / maker

strategy:
  leaders:
    - "okx"
    - "binance"

  entry_threshold_bps: 15        # θ_entry：15 bps = 0.15%
  spread_persist_ms: 200         # 100–300ms 之间可配
  cooldown_after_sl_ms: 3000     # 3–5s
  max_hold_ms: 15000            # 10–25s

  depth_filter:
    enable: true
    leader_min_top5_usd: 50000  # 例：Leader 前5档合计>=5万U

  vol_filter:
    enable: true
    window_sec: 60
    max_abs_return: 0.02        # 1分钟波动>2%则跳过

paper:
  enable: true
  tp_model:
    type: "spread_ratio"
    r_tp: 1.0
  sl_model:
    type: "spread_ratio"
    r_sl: 1.0

output:
  log_level: "info"
  metrics_interval_ms: 1000
  snapshot_interval_ms: 100
  write_jsonl: true
  path: "./data"
  files:
    lag: "lag_stats.jsonl"
    signals: "signals.jsonl"
    paper: "paper_trades.jsonl"
```

---

## 11. 开发落地要点（给 AI 工程师的“坑位提示”）

1. **Binance 推送里有 `E`，建议写入 `ExchTsUnixMs`**，否则你只能做 arrival-based lag，解释力会弱。
2. **Bittap 无 server ts**，所以“绝对延迟”只能近似；重点看**分布稳定性**和“跟随概率”。
3. **必须做重连退避**，否则网络抖动会在统计里产生大量假异常点。
4. **必须分开统计 OKX->Bittap 与 Binance->Bittap**（你已确认两者都是 leader 且独立统计）。
5. 影子成交不要太乐观：默认 `taker->taker`，先验证下限能否覆盖手续费压缩风险。

---

## 12. 你还“可选提供”的信息（非必须，但会让验证结论更硬）

* Bittap 只支持 f_depth30。
* Bittap 的 `lastUpdateId` 是否严格递增、是否会重置（用于断线后去重逻辑）。
* 你想重点验证的币种列表与各币种的 `bittap_tick`（档位 i）。

---

