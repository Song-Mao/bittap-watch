# Implementation Plan

- [x] 1. Initialize project structure and dependencies






  - [x] 1.1 Create Go module and directory structure


    - Initialize `go.mod` with module name `latency-arbitrage-validator`
    - Create directory structure as specified in design: `cmd/validator/`, `internal/config/`, `internal/metadata/`, `internal/exchange/{okx,binance,bittap}/`, `internal/core/{model,store,signal,paper}/`, `internal/stats/{ev,latency}/`, `internal/output/jsonl/`, `internal/util/{backoff,timeutil,fastparse}/`


    - _Requirements: 2.1 (Go 工程标准)_
  - [x] 1.2 Add core dependencies





    - Add `gopkg.in/yaml.v3` for config parsing
    - Add `github.com/gorilla/websocket` for WebSocket connections
    - Add `go.uber.org/zap` for structured logging
    - Add `github.com/leanovate/gopter` for property-based testing
    - _Requirements: 2.1_

- [x] 2. Implement configuration module



  - [x] 2.1 Create config types and loader


    - Implement `Config` struct with all nested config types (AppConfig, SymbolConfig, WSConfig, FeesConfig, StrategyConfig, PaperConfig, OutputConfig)
    - Implement `Load(path string) (*Config, error)` function
    - Implement `Validate() error` method with all validation rules
    - 所有导出类型和函数必须包含中文 GoDoc 注释
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 12.1, 12.3_
  - [x] 2.2 Write property test for config validation


    - **Property 20: Config Validation Correctness**
    - **Validates: Requirements 9.2, 9.3, 9.4, 9.5**

- [x] 3. Implement utility modules



  - [x] 3.1 Implement exponential backoff


    - Create `Backoff` struct with base, max, jitter parameters
    - Implement `Next() time.Duration` with exponential growth and jitter
    - Implement `Reset()` method
    - 添加中文注释说明退避算法逻辑
    - _Requirements: 1.3, 12.1, 12.2_
  - [x] 3.2 Write property test for backoff


    - **Property 3: Exponential Backoff Bounds**
    - **Validates: Requirements 1.3**
  - [x] 3.3 Implement fast parse utilities


    - Implement `ParseFloat(s string) (float64, error)` using `strconv.ParseFloat`
    - Implement `ParseInt(s string) (int64, error)` using `strconv.ParseInt`
    - 添加中文注释说明解析函数用途
    - _Requirements: 3.3 (分配与 GC 控制), 12.1_

  - [x] 3.4 Implement time utilities

    - Implement `NowNano() int64` for monotonic nanosecond timestamps
    - Implement `NowMs() int64` for millisecond timestamps
    - 添加中文注释说明时间戳用途
    - _Requirements: 2.2, 12.1_

- [x] 4. Implement core data models



  - [x] 4.1 Create BookEvent and related types

    - Implement `BookEvent` struct with all fields (Exchange, SymbolCanon, BestBidPx, BestBidQty, BestAskPx, BestAskQty, Levels, ArrivedAtUnixNs, ExchTsUnixMs, Seq)
    - Implement `Level` struct (Price, Qty)
    - Implement `Side` type with Long/Short constants
    - 每个结构体字段必须包含中文注释说明用途
    - _Requirements: 2.1, 4.1 (统一结构), 12.1, 12.3_

  - [x] 4.2 Create Signal and Position types

    - Implement `Signal` struct (Leader, SymbolCanon, Side, SpreadBps, LeaderBook, FollowerBook, DetectedAt)
    - Implement `Position` struct with all paper trade fields
    - Implement `ExitReason` type with TP/SL/Timeout constants
    - 每个结构体字段必须包含中文注释说明用途
    - _Requirements: 5.1, 5.2, 6.1, 12.1, 12.3_

- [x] 5. Implement metadata module



  - [x] 5.1 Create metadata types for each exchange

    - Implement `OKXInstrument` struct matching API response
    - Implement `BinanceSymbol` struct matching API response
    - Implement `BittapExchangeInfo` struct matching API response
    - 添加中文注释映射 API 响应字段
    - _Requirements: 3.1, 12.3, 12.5_
  - [x] 5.2 Implement metadata fetcher


    - Implement `Fetcher` interface with FetchOKX, FetchBinance, FetchBittap methods
    - Implement HTTP client with timeout and error handling
    - 添加中文注释说明获取逻辑
    - _Requirements: 3.1, 12.1_
  - [x] 5.3 Implement symbol mapper


    - Implement `SymbolMap` struct (Canon, OKXInstId, BinanceSym, BittapSym, BittapTick, TickSize)
    - Implement `BuildSymbolMaps()` function to match user input to exchange symbols
    - Handle USDT perpetual contract filtering (OKX: ctType=linear, settleCcy=USDT; Binance: contractType=PERPETUAL)
    - 添加中文注释说明映射逻辑
    - _Requirements: 3.2, 3.3, 3.4, 3.5, 12.1, 12.2_
  - [x] 5.4 Write property test for symbol normalization


    - **Property 2: Symbol Normalization Consistency**
    - **Validates: Requirements 2.6, 3.2**

- [x] 6. Checkpoint - Ensure all tests pass



  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Implement exchange adapters - OKX


  - [x] 7.1 Implement OKX WebSocket client

    - Implement connection with headers (Origin, User-Agent)
    - Implement subscription message builder for books5 channel
    - Implement read loop with context cancellation
    - 添加中文注释说明连接和订阅逻辑
    - _Requirements: 1.1, 1.2, 12.1, 12.2_

  - [x] 7.2 Implement OKX message parser
    - Parse books5 response to BookEvent
    - Extract `ts` as ExchTsUnixMs, `seqId` as Seq
    - Handle bids/asks array parsing with fastparse
    - 添加中文注释映射 OKX 响应字段到 BookEvent

    - _Requirements: 2.1, 2.3, 12.5_
  - [x] 7.3 Implement OKX heartbeat handler
    - Send text `ping` at 25s intervals
    - Expect `pong` within 10s, trigger reconnect on timeout
    - Track WsRttMs metric
    - 添加中文注释说明心跳机制

    - _Requirements: 1.4, 12.2_
  - [x] 7.4 Write property test for OKX parser

    - **Property 1: Parser Round-Trip Consistency (OKX)**
    - **Validates: Requirements 2.1, 2.3**

- [x] 8. Implement exchange adapters - Binance

  - [x] 8.1 Implement Binance WebSocket client


    - Implement connection to fstream endpoint
    - Implement subscription message builder for depth5@100ms
    - Implement read loop with protocol-level ping/pong
    - 添加中文注释说明连接和订阅逻辑
    - _Requirements: 1.1, 1.2, 12.1, 12.2_
  - [x] 8.2 Implement Binance message parser
    - Parse depthUpdate response to BookEvent
    - Extract `E` as ExchTsUnixMs, set Seq=0
    - Handle bids/asks array parsing
    - 添加中文注释映射 Binance 响应字段到 BookEvent
    - _Requirements: 2.1, 2.4, 12.5_
  - [x] 8.3 Write property test for Binance parser
    - **Property 1: Parser Round-Trip Consistency (Binance)**
    - **Validates: Requirements 2.1, 2.4**

- [x] 9. Implement exchange adapters - Bittap
  - [x] 9.1 Implement Bittap WebSocket client
    - Implement connection with JSON format parameter
    - Implement subscription message builder for f_depth30 channel with tick
    - Implement read loop with context cancellation
    - 添加中文注释说明连接和订阅逻辑
    - _Requirements: 1.1, 1.2, 12.1, 12.2_
  - [x] 9.2 Implement Bittap message parser
    - Parse f_depth30 response to BookEvent
    - Set ExchTsUnixMs=0, extract `lastUpdateId` as Seq
    - Handle bids/asks array parsing
    - 添加中文注释映射 Bittap 响应字段到 BookEvent
    - _Requirements: 2.1, 2.5, 12.5_
  - [x] 9.3 Implement Bittap heartbeat handler
    - Send JSON `{"method":"PING"}` at 18s intervals
    - Handle PONG response
    - 添加中文注释说明心跳机制
    - _Requirements: 1.5, 12.2_
  - [x] 9.4 Write property test for Bittap parser
    - **Property 1: Parser Round-Trip Consistency (Bittap)**
    - **Validates: Requirements 2.1, 2.5**

- [x] 10. Implement reconnection logic
  - [x] 10.1 Add reconnection to all adapters
    - Integrate backoff module for reconnection delays
    - Handle connection errors, read errors, heartbeat timeouts
    - Track reconnect_count metric
    - _Requirements: 1.3, 1.6_
  - [x] 10.2 Implement connection metrics
    - Track parse_error_count with rate-limited logging
    - Track updates_per_sec per exchange/symbol
    - Track last_message_age_ms
    - _Requirements: 1.6_

- [x] 11. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 12. Implement book store
  - [x] 12.1 Create book store with single-writer pattern
    - Implement `Store` struct with nested maps (exchange -> symbol -> BookEvent)
    - Implement `Update(event *BookEvent)` method
    - Implement `Get(exchange, symbol string) *BookEvent` method
    - Implement `GetPair(leader, symbol string) (leaderBook, followerBook *BookEvent)` method
    - 添加中文注释说明单写者模式设计
    - _Requirements: 3.2 (禁止共享可变状态), 12.1, 12.2_

- [x] 13. Implement latency tracker
  - [x] 13.1 Create latency statistics module
    - Implement `Tracker` struct with rolling sample storage
    - Implement `Add(lagMs float64)` method
    - Implement `Stats() *LatencyStats` with P50/P90/P99 calculation
    - Maintain separate trackers for OKX→Bittap and Binance→Bittap
    - 添加中文注释说明延迟统计算法
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 12.1, 12.2_
  - [x] 13.2 Write property test for lag calculation
    - **Property 4: Lag Calculation Correctness**
    - **Validates: Requirements 4.1, 4.2**
  - [x] 13.3 Write property test for percentile calculation
    - **Property 5: Percentile Calculation Correctness**
    - **Validates: Requirements 4.3**
  - [x] 13.4 Write property test for leader independence
    - **Property 6: Leader Metrics Independence**
    - **Validates: Requirements 4.4**

- [x] 14. Implement signal engine
  - [x] 14.1 Create signal detection logic
    - Implement spread calculation: long = leader_bid - follower_ask, short = follower_bid - leader_ask
    - Implement threshold comparison with θ_entry
    - Generate Signal struct on detection
    - 添加中文注释说明价差计算公式
    - _Requirements: 5.1, 5.2, 12.2, 12.4_
  - [x] 14.2 Write property test for signal detection
    - **Property 7: Signal Detection Correctness**
    - **Validates: Requirements 5.1, 5.2**
  - [x] 14.3 Implement persistence filter
    - Track spread history with timestamps
    - Require spread > θ_entry for persist_ms duration
    - 添加中文注释说明持续时间过滤逻辑
    - _Requirements: 5.3, 12.2_
  - [x] 14.4 Write property test for persistence filter
    - **Property 8: Persistence Filter Correctness**
    - **Validates: Requirements 5.3**
  - [x] 14.5 Implement depth filter
    - Calculate top-5 depth USD value from Leader book
    - Compare against configured minimum
    - 添加中文注释说明深度过滤逻辑
    - _Requirements: 5.4, 12.2_
  - [x] 14.6 Write property test for depth filter
    - **Property 9: Depth Filter Correctness**
    - **Validates: Requirements 5.4**
  - [x] 14.7 Implement volatility filter
    - Track 1-minute price returns
    - Calculate realized volatility
    - Skip signals when volatility exceeds threshold
    - 添加中文注释说明波动率计算公式
    - _Requirements: 5.5, 12.2, 12.4_
  - [x] 14.8 Write property test for volatility filter
    - **Property 10: Volatility Filter Correctness**
    - **Validates: Requirements 5.5**
  - [x] 14.9 Implement cooldown tracker
    - Track last SL exit time per leader-symbol pair
    - Block signals within cooldown_ms of SL
    - 添加中文注释说明冷却机制
    - _Requirements: 5.6, 12.2_
  - [x] 14.10 Write property test for cooldown
    - **Property 11: Cooldown Enforcement**
    - **Validates: Requirements 5.6**

- [x] 15. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 16. Implement paper executor
  - [x] 16.1 Create paper position management
    - Implement `Open(sig *Signal) *Position` with entry price from Follower book
    - Track open positions by ID
    - 添加中文注释说明开仓逻辑
    - _Requirements: 6.1, 12.1, 12.2_
  - [x] 16.2 Write property test for entry price
    - **Property 12: Paper Entry Price Correctness**
    - **Validates: Requirements 6.1**
  - [x] 16.3 Implement fee calculation
    - Calculate effective_fee = raw_fee × (1 - rebate_rate)
    - Calculate total fee_bps for entry + exit
    - 添加中文注释说明手续费计算公式
    - _Requirements: 6.2, 12.2, 12.4_
  - [x] 16.4 Write property test for fee calculation
    - **Property 13: Fee Calculation Correctness**
    - **Validates: Requirements 6.2**
  - [x] 16.5 Implement exit conditions
    - Implement TP check: |current_spread| ≤ (1 - r_tp) × |entry_spread|
    - Implement SL check: |current_spread| ≥ (1 + r_sl) × |entry_spread|
    - Implement timeout check: hold_time > max_hold_ms
    - 添加中文注释说明 TP/SL/超时退出条件公式
    - _Requirements: 6.3, 6.4, 6.5, 12.2, 12.4_
  - [x] 16.6 Write property test for exit conditions
    - **Property 14: Position Exit Correctness**
    - **Validates: Requirements 6.3, 6.4, 6.5**
  - [x] 16.7 Implement PnL calculation
    - Calculate gross_pnl_bps = (exit_px - entry_px) / entry_px × 10000 × direction
    - Calculate net_pnl_bps = gross_pnl_bps - fee_bps
    - 添加中文注释说明 PnL 计算公式
    - _Requirements: 6.6, 12.2, 12.4_
  - [x] 16.8 Write property test for PnL calculation
    - **Property 15: PnL Calculation Correctness**
    - **Validates: Requirements 6.6**

- [x] 17. Implement EV calculator
  - [x] 17.1 Create rolling statistics tracker
    - Track wins/losses count
    - Calculate rolling win rate, avg profit, avg loss
    - 添加中文注释说明滚动统计算法
    - _Requirements: 7.1, 12.1, 12.2_
  - [x] 17.2 Write property test for rolling statistics
    - **Property 16: Rolling Statistics Correctness**
    - **Validates: Requirements 7.1**
  - [x] 17.3 Implement EV formula
    - Calculate EV = p × (R - f) + (1 - p) × (-L - f)
    - Calculate p_required = (L + f) / (R + L)
    - 添加中文注释详细说明 EV 计算公式
    - _Requirements: 7.2, 7.3, 12.2, 12.4_
  - [x] 17.4 Write property test for EV formula
    - **Property 17: EV Formula Correctness**
    - **Validates: Requirements 7.2, 7.3**
  - [x] 17.5 Implement EV rejection
    - Mark signals as RejectedByEV when EV < 0
    - 添加中文注释说明 EV 拒绝逻辑
    - _Requirements: 7.4, 12.2_
  - [x] 17.6 Write property test for EV rejection
    - **Property 18: EV Rejection Correctness**
    - **Validates: Requirements 7.4**

- [x] 18. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 19. Implement output module
  - [x] 19.1 Create async JSONL writer
    - Implement buffered channel for async writes
    - Implement writer goroutine with file I/O
    - Implement `Write(v interface{})`, `Flush()`, `Close()` methods
    - 添加中文注释说明异步写入机制
    - _Requirements: 8.1, 8.2, 8.3, 12.1, 12.2_
  - [x] 19.2 Implement paper trade output format
    - Ensure all required fields are included in JSON output
    - 添加中文注释说明输出字段
    - _Requirements: 8.4, 12.3_
  - [x] 19.3 Write property test for output completeness
    - **Property 19: Paper Trade Output Completeness**
    - **Validates: Requirements 8.4**

- [x] 20. Implement main aggregator
  - [x] 20.1 Create aggregator goroutine
    - Receive BookEvents from all three exchange channels
    - Update book store (single writer)
    - Trigger signal engines on updates
    - 添加中文注释说明聚合器架构
    - _Requirements: 3.1 (Goroutine/Channel 架构), 12.1, 12.2_
  - [x] 20.2 Wire signal engines to paper executor
    - Pass signals through filters
    - Open/evaluate/close paper positions
    - Send results to output writer
    - 添加中文注释说明信号处理流程
    - _Requirements: 5.1-5.6, 6.1-6.6, 12.2_

- [x] 21. Implement graceful shutdown
  - [x] 21.1 Add signal handling
    - Handle SIGINT and SIGTERM
    - Propagate cancellation via context
    - 添加中文注释说明信号处理
    - _Requirements: 10.1, 12.2_
  - [x] 21.2 Implement shutdown sequence
    - Close all WebSocket connections
    - Flush all output writers
    - Output final metrics summary
    - Enforce 10-second timeout
    - 添加中文注释说明关闭顺序
    - _Requirements: 10.2, 10.3, 10.4, 10.5, 12.2_

- [x] 22. Create main entry point
  - [x] 22.1 Implement cmd/validator/main.go
    - Parse command-line flags (--config)
    - Load and validate configuration
    - Fetch metadata and build symbol maps
    - Initialize all components
    - Start goroutines and wait for shutdown
    - 添加中文注释说明启动流程
    - _Requirements: 9.1, 10.1, 12.1, 12.2_

- [x] 23. Create sample configuration
  - [x] 23.1 Create config.yaml template
    - Include all configurable parameters with sensible defaults
    - Add comments explaining each parameter
    - _Requirements: 8.1 (配置文件规范)_

- [x] 24. Final Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
