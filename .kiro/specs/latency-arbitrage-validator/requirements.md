# Requirements Document

## Introduction

本系统是一个加密货币 USDT 永续合约延迟套利验证器，用于测量和验证 OKX、Binance（领先交易所）与 Bittap（跟随交易所）之间的价格传播延迟，以确定套利机会是否能覆盖手续费和滑点。

系统处于研究/验证阶段：
- 不进行真实下单
- 仅采集公共行情数据
- 通过影子成交（Paper Execution）模拟 PnL
- 部署于 GCP Tokyo，单进程，IPv4，4 核

## Glossary

- **Leader（领先交易所）**: OKX 或 Binance，价格发现更快的交易所
- **Follower（跟随交易所）**: Bittap，报价调整较慢的交易所
- **BookEvent**: 统一的订单簿事件结构，包含最优买卖价、深度、时间戳等
- **SymbolCanon**: 内部统一的交易对标识符（如 `BTCUSDT`）
- **Lead-Lag**: 领先交易所与跟随交易所之间的报价延迟
- **Paper Execution（影子成交）**: 不实际下单，但模拟以当时可成交价格进出的 PnL
- **θ_entry**: 入场阈值，价差超过此值才触发信号
- **TP（Take Profit）**: 止盈，价差收敛时退出
- **SL（Stop Loss）**: 止损，价差发散时退出
- **EV（Expected Value）**: 每笔交易的预期回报
- **ArrivedAt**: 本机收到消息的时间戳（纳秒）
- **ExchTs**: 交易所事件时间戳（毫秒）
- **GoDoc**: Go 语言官方文档注释风格，用于生成 API 文档

## Requirements

### Requirement 1

**User Story:** As a quantitative researcher, I want to connect to multiple exchange WebSocket feeds simultaneously, so that I can receive real-time order book data for latency analysis.

#### Acceptance Criteria

1. WHEN the Validator starts, THE Validator SHALL establish WebSocket connections to OKX, Binance, and Bittap public market data endpoints within 10 seconds
2. WHEN a WebSocket connection is established, THE Validator SHALL subscribe to order book depth channels for configured symbols
3. WHEN a WebSocket connection fails or disconnects, THE Validator SHALL automatically reconnect using exponential backoff with base 1 second, maximum 30 seconds, and ±20% jitter
4. WHEN OKX WebSocket requires heartbeat, THE Validator SHALL send text ping messages at 25-second intervals and expect pong responses within 10 seconds
5. WHEN Bittap WebSocket requires heartbeat, THE Validator SHALL send JSON PING messages at 18-second intervals
6. WHILE a WebSocket connection is active, THE Validator SHALL record connection quality metrics including reconnect count, parse error count, and updates per second

### Requirement 2

**User Story:** As a quantitative researcher, I want order book data from all exchanges normalized to a unified format, so that I can perform consistent cross-exchange analysis.

#### Acceptance Criteria

1. WHEN a raw WebSocket message is received, THE Validator SHALL parse the message and convert it to a unified BookEvent structure
2. WHEN creating a BookEvent, THE Validator SHALL record the local arrival timestamp in nanoseconds using monotonic clock
3. WHEN parsing OKX books5 messages, THE Validator SHALL extract ts as ExchTsUnixMs and seqId as Seq
4. WHEN parsing Binance depth5 messages, THE Validator SHALL extract E (event time) as ExchTsUnixMs
5. WHEN parsing Bittap f_depth30 messages, THE Validator SHALL set ExchTsUnixMs to 0 and extract lastUpdateId as Seq
6. WHEN normalizing symbols, THE Validator SHALL convert exchange-specific symbols to canonical format (e.g., BTC-USDT → BTCUSDT)

### Requirement 3

**User Story:** As a quantitative researcher, I want the system to fetch and map contract metadata at startup, so that I can subscribe to the correct channels without hardcoding symbols.

#### Acceptance Criteria

1. WHEN the Validator starts, THE Validator SHALL fetch contract metadata from OKX, Binance, and Bittap REST APIs
2. WHEN metadata is fetched, THE Validator SHALL build a symbol mapping table that maps user input (e.g., BTC-USDT) to exchange-specific identifiers
3. WHEN a user-configured symbol cannot be mapped to all three exchanges, THE Validator SHALL fail fast with a descriptive error message
4. WHEN building WebSocket subscription messages, THE Validator SHALL use mapped symbols from the metadata, not hardcoded values
5. WHEN metadata contains tick size information, THE Validator SHALL store it for Bittap subscription channel construction (f_depth30@{symbol}_{tick})

### Requirement 4

**User Story:** As a quantitative researcher, I want to measure lead-lag timing between exchanges, so that I can understand price propagation delays.

#### Acceptance Criteria

1. WHEN BookEvents are received from both Leader and Follower, THE Validator SHALL calculate arrival-based lag as Follower_ArrivedAt minus Leader_ArrivedAt
2. WHEN Leader has valid ExchTs, THE Validator SHALL calculate event-based lag as Follower_ArrivedAt minus Leader_ExchTs
3. WHILE collecting lag measurements, THE Validator SHALL maintain rolling statistics for P50, P90, and P99 percentiles
4. WHEN outputting lag statistics, THE Validator SHALL separate OKX→Bittap and Binance→Bittap metrics independently

### Requirement 5

**User Story:** As a quantitative researcher, I want to detect arbitrage signal opportunities based on price spreads, so that I can evaluate potential trading strategies.

#### Acceptance Criteria

1. WHEN Leader best bid exceeds Follower best ask by more than θ_entry, THE Validator SHALL generate a long signal
2. WHEN Follower best bid exceeds Leader best ask by more than θ_entry, THE Validator SHALL generate a short signal
3. WHEN a signal is detected, THE Validator SHALL apply persistence filter requiring the spread to persist for configurable duration (100-300ms)
4. WHEN a signal is detected, THE Validator SHALL apply depth filter requiring Leader top-5 depth to exceed configurable minimum USD value
5. WHEN 1-minute realized volatility exceeds configurable threshold, THE Validator SHALL skip signal generation
6. WHEN a stop-loss exit occurs, THE Validator SHALL apply cooldown period (3-5 seconds) before generating new signals

### Requirement 6

**User Story:** As a quantitative researcher, I want to simulate paper trades based on detected signals, so that I can evaluate strategy profitability without real orders.

#### Acceptance Criteria

1. WHEN a signal passes all filters, THE Validator SHALL create a paper position using Bittap best ask (for long) or best bid (for short) as entry price
2. WHEN calculating paper trade costs, THE Validator SHALL apply configurable taker fee rate with rebate adjustment
3. WHEN the spread converges below (1 - r_tp) × initial spread, THE Validator SHALL close the paper position as take-profit
4. WHEN the spread diverges above (1 + r_sl) × initial spread, THE Validator SHALL close the paper position as stop-loss
5. WHEN a paper position exceeds maximum hold time, THE Validator SHALL force close the position as timeout
6. WHEN a paper trade closes, THE Validator SHALL calculate and record gross PnL, fee, and net PnL in basis points

### Requirement 7

**User Story:** As a quantitative researcher, I want to calculate expected value metrics, so that I can determine if the strategy is statistically profitable.

#### Acceptance Criteria

1. WHILE paper trades accumulate, THE Validator SHALL maintain rolling estimates of win rate (p), average profit (R), and average loss (L)
2. WHEN calculating EV, THE Validator SHALL use formula: EV = p × (R - f) + (1 - p) × (-L - f)
3. WHEN calculating breakeven win rate, THE Validator SHALL use formula: p_required = (L + f) / (R + L)
4. WHEN EV is negative, THE Validator SHALL mark subsequent signals as RejectedByEV

### Requirement 8

**User Story:** As a quantitative researcher, I want all statistics and paper trades persisted to files, so that I can analyze results offline and reproduce findings.

#### Acceptance Criteria

1. WHILE the Validator runs, THE Validator SHALL write signal events to signals.jsonl asynchronously
2. WHILE the Validator runs, THE Validator SHALL write paper trade results to paper_trades.jsonl asynchronously
3. WHILE the Validator runs, THE Validator SHALL write metrics snapshots to metrics.jsonl at configurable intervals
4. WHEN writing paper_trades, THE Validator SHALL include leader, symbol_canon, side, entry/exit timestamps, entry/exit prices, gross_pnl_bps, fee_bps, net_pnl_bps, and exit_reason
5. WHEN the Validator shuts down, THE Validator SHALL flush all pending writes before exiting

### Requirement 9

**User Story:** As a system operator, I want the system to be configurable via YAML file, so that I can adjust parameters without code changes.

#### Acceptance Criteria

1. WHEN the Validator starts, THE Validator SHALL load configuration from config.yaml file
2. WHEN loading configuration, THE Validator SHALL validate that fee rates are within range 0 to 1
3. WHEN loading configuration, THE Validator SHALL validate that θ_entry, persist_ms, and max_hold_ms have reasonable positive values
4. WHEN loading configuration, THE Validator SHALL validate that at least one symbol is configured and mappable
5. IF configuration validation fails, THEN THE Validator SHALL exit with descriptive error message

### Requirement 10

**User Story:** As a system operator, I want the system to shut down gracefully, so that no data is lost and resources are properly released.

#### Acceptance Criteria

1. WHEN receiving SIGINT or SIGTERM, THE Validator SHALL initiate graceful shutdown
2. WHEN shutting down, THE Validator SHALL close all WebSocket connections
3. WHEN shutting down, THE Validator SHALL flush all output writers
4. WHEN shutting down, THE Validator SHALL output final metrics summary
5. WHEN shutting down, THE Validator SHALL complete within 10 seconds or force exit

### Requirement 11

**User Story:** As a compliance officer, I want to ensure the system never places real orders, so that we remain in research-only mode.

#### Acceptance Criteria

1. THE Validator SHALL NOT contain any code for placing, modifying, or canceling orders
2. THE Validator SHALL NOT contain any code for private WebSocket authentication or login
3. THE Validator SHALL NOT contain any code for API key signing or secret management
4. THE Validator SHALL only use public market data WebSocket endpoints and public REST metadata endpoints

### Requirement 12

**User Story:** As a Chinese-speaking developer, I want all code to have Chinese comments, so that I can easily understand and maintain the codebase.

#### Acceptance Criteria

1. WHEN writing exported functions or types, THE Validator SHALL include Chinese GoDoc-style comments explaining the purpose and usage
2. WHEN writing complex logic or algorithms, THE Validator SHALL include inline Chinese comments explaining the implementation details
3. WHEN defining struct fields, THE Validator SHALL include Chinese comments describing each field's purpose
4. WHEN implementing state machines or formulas (ΔP, TP/SL, fee, EV), THE Validator SHALL include Chinese comments explaining the calculation logic
5. WHEN parsing exchange messages, THE Validator SHALL include Chinese comments mapping response fields to struct fields
