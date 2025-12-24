// Package binance 实现 Binance 交易所的 WebSocket 客户端。
// 连接地址: wss://fstream.binance.com/ws
// 订阅频道: depth5@100ms
// 心跳机制: 协议层 ping/pong
package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/metadata"
	"latency-arbitrage-validator/internal/util/backoff"
	"latency-arbitrage-validator/internal/util/timeutil"
)

// Client Binance WebSocket 客户端
type Client struct {
	// cfg WebSocket 配置
	cfg *config.ExchangeWSConfig
	// symbolMaps Symbol 映射表（key 为 Canon）
	symbolMaps map[string]*metadata.SymbolMap
	// logger 日志记录器
	logger *zap.Logger
	// parser 消息解析器
	parser *Parser

	// conn WebSocket 连接
	conn *websocket.Conn
	// connMu 连接锁
	connMu sync.Mutex

	// bookCh 订单簿事件输出通道
	bookCh chan *model.BookEvent
	// errCh 错误输出通道
	errCh chan error

	// metrics 连接指标
	metrics ConnectionMetrics
	// metricsMu 指标锁
	metricsMu sync.RWMutex

	// lastMsgTime 最后消息时间（纳秒）
	lastMsgTime int64
	// updateCount 更新计数（用于计算 QPS）
	updateCount int64
	// backoff 重连退避
	backoff *backoff.Backoff
	// closed 是否已关闭
	closed int32

	// parseErrSampleCount 解析错误计数（用于采样日志）
	parseErrSampleCount uint64
	// lastParseErrLogNs 上次解析错误日志时间（纳秒）
	lastParseErrLogNs int64
}

// NewClient 创建 Binance WebSocket 客户端
// 参数 cfg: WebSocket 配置
// 参数 symbolMaps: Symbol 映射表（key 为 Canon）
// 参数 logger: 日志记录器
func NewClient(cfg *config.ExchangeWSConfig, symbolMaps map[string]*metadata.SymbolMap, logger *zap.Logger) *Client {
	return &Client{
		cfg:        cfg,
		symbolMaps: symbolMaps,
		logger:     logger.Named("binance"),
		parser:     NewParser(symbolMaps),
		bookCh:     make(chan *model.BookEvent, 1000),
		errCh:      make(chan error, 10),
		backoff:    backoff.NewDefault(),
	}
}

// Connect 建立 WebSocket 连接
// 参数 ctx: 上下文，用于取消连接
func (c *Client) Connect(ctx context.Context) error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	header := http.Header{}
	header.Set("User-Agent", "latency-arbitrage-validator/1.0")
	header.Set("Origin", "https://www.binance.com")

	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, c.cfg.URL, header)
	if err != nil {
		return fmt.Errorf("连接 Binance WebSocket 失败: %w", err)
	}

	readTimeout := time.Duration(c.readTimeoutMs()) * time.Millisecond
	if readTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		conn.SetPongHandler(func(string) error {
			atomic.StoreInt64(&c.lastMsgTime, timeutil.NowNano())
			return conn.SetReadDeadline(time.Now().Add(readTimeout))
		})
	}

	c.conn = conn
	c.backoff.Reset()
	c.logger.Info("Binance WebSocket 连接成功", zap.String("url", c.cfg.URL))
	return nil
}

// Subscribe 订阅交易对
// 订阅 depth5@100ms 行情流
func (c *Client) Subscribe() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("WebSocket 未连接")
	}

	params := make([]string, 0, len(c.symbolMaps))
	for _, m := range c.symbolMaps {
		// Binance 订阅参数要求小写 symbol
		params = append(params, fmt.Sprintf("%s@depth5@100ms", strings.ToLower(m.BinanceSym)))
	}

	req := SubscribeRequest{
		Method: "SUBSCRIBE",
		Params: params,
		ID:     1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化订阅请求失败: %w", err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("发送订阅请求失败: %w", err)
	}

	c.logger.Info("Binance 订阅请求已发送", zap.Int("symbols", len(params)))
	return nil
}

// Run 启动客户端主循环
// 包含读取循环和指标统计
func (c *Client) Run(ctx context.Context) {
	go c.pingLoop(ctx)
	go c.metricsLoop(ctx)
	c.readLoop(ctx)
}

func (c *Client) readLoop(ctx context.Context) {
	readTimeout := time.Duration(c.readTimeoutMs()) * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if atomic.LoadInt32(&c.closed) == 1 {
			return
		}

		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()

		if conn == nil {
			c.reconnect(ctx)
			continue
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			c.logger.Warn("读取 Binance 消息失败", zap.Error(err))
			c.incrementReconnectCount()
			c.reconnect(ctx)
			continue
		}

		if readTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
		}

		atomic.StoreInt64(&c.lastMsgTime, timeutil.NowNano())

		events, err := c.parser.Parse(data)
		if err != nil {
			c.incrementParseErrorCount()
			c.maybeLogParseError(err, data)
			continue
		}

		for _, event := range events {
			atomic.AddInt64(&c.updateCount, 1)
			select {
			case c.bookCh <- event:
			default:
				c.logger.Warn("Binance bookCh 已满，丢弃事件")
			}
		}
	}
}

func (c *Client) pingLoop(ctx context.Context) {
	intervalMs := c.cfg.PingIntervalMs
	if intervalMs <= 0 {
		intervalMs = c.readTimeoutMs() / 2
		if intervalMs <= 0 {
			intervalMs = 15000
		}
	}

	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if atomic.LoadInt32(&c.closed) == 1 {
				return
			}

			c.connMu.Lock()
			conn := c.conn
			if conn == nil {
				c.connMu.Unlock()
				continue
			}

			deadline := time.Now().Add(5 * time.Second)
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), deadline); err != nil {
				c.connMu.Unlock()
				c.logger.Warn("发送 Binance ping 失败", zap.Error(err))
				continue
			}
			c.connMu.Unlock()
		}
	}
}

func (c *Client) metricsLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var lastCount int64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if atomic.LoadInt32(&c.closed) == 1 {
				return
			}

			count := atomic.LoadInt64(&c.updateCount)
			qps := float64(count - lastCount)
			lastCount = count

			lastMsg := atomic.LoadInt64(&c.lastMsgTime)
			var ageMs int64
			if lastMsg > 0 {
				ageMs = (timeutil.NowNano() - lastMsg) / 1_000_000
			}

			c.metricsMu.Lock()
			c.metrics.UpdatesPerSec = qps
			c.metrics.LastMessageAgeMs = ageMs
			c.metricsMu.Unlock()
		}
	}
}

func (c *Client) reconnect(ctx context.Context) {
	c.closeConn()

	delay := c.backoff.Next()
	c.logger.Info("Binance 准备重连", zap.Duration("delay", delay))

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	if err := c.Connect(ctx); err != nil {
		c.logger.Error("Binance 重连失败", zap.Error(err))
		return
	}
	if err := c.Subscribe(); err != nil {
		c.logger.Error("Binance 重新订阅失败", zap.Error(err))
	}
}

func (c *Client) closeConn() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// Close 关闭客户端
func (c *Client) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	c.closeConn()
	close(c.bookCh)
	close(c.errCh)
	c.logger.Info("Binance 客户端已关闭")
	return nil
}

// BookCh 获取订单簿事件通道
func (c *Client) BookCh() <-chan *model.BookEvent {
	return c.bookCh
}

// ErrCh 获取错误通道
func (c *Client) ErrCh() <-chan error {
	return c.errCh
}

// Metrics 获取连接指标
func (c *Client) Metrics() ConnectionMetrics {
	c.metricsMu.RLock()
	defer c.metricsMu.RUnlock()
	return c.metrics
}

func (c *Client) incrementReconnectCount() {
	c.metricsMu.Lock()
	c.metrics.ReconnectCount++
	c.metricsMu.Unlock()
}

func (c *Client) incrementParseErrorCount() {
	c.metricsMu.Lock()
	c.metrics.ParseErrorCount++
	c.metricsMu.Unlock()
}

func (c *Client) readTimeoutMs() int {
	if c.cfg.ReadTimeoutMs > 0 {
		return c.cfg.ReadTimeoutMs
	}
	// 未配置时使用 30s
	return 30000
}

// maybeLogParseError 采样记录解析错误原始消息，避免刷盘
// 采样策略：每 100 次错误记录 1 条，且同一类日志至少间隔 1 分钟。
func (c *Client) maybeLogParseError(err error, data []byte) {
	count := atomic.AddUint64(&c.parseErrSampleCount, 1)
	if count%100 != 0 {
		return
	}

	nowNs := timeutil.NowNano()
	last := atomic.LoadInt64(&c.lastParseErrLogNs)
	if last > 0 && nowNs-last < int64(time.Minute) {
		return
	}
	atomic.StoreInt64(&c.lastParseErrLogNs, nowNs)

	sample := data
	if len(sample) > 200 {
		sample = sample[:200]
	}
	c.logger.Warn("解析 Binance 消息失败（采样）", zap.Error(err), zap.ByteString("data", sample))
}
