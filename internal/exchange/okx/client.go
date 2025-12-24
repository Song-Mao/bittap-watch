// Package okx 实现 OKX 交易所的 WebSocket 客户端。
// 连接地址: wss://ws.okx.com:8443/ws/v5/public
// 订阅频道: books5
// 心跳机制: 文本 ping/pong，25秒间隔，10秒超时
package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// Client OKX WebSocket 客户端
type Client struct {
	// cfg WebSocket 配置
	cfg *config.ExchangeWSConfig
	// symbolMaps Symbol 映射表
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
	// lastMsgTime 最后消息时间
	lastMsgTime int64
	// lastPingSentNs 上次发送 ping 的时间（纳秒）
	lastPingSentNs int64
	// lastPongRecvNs 上次收到 pong 的时间（纳秒）
	lastPongRecvNs int64
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

// NewClient 创建 OKX WebSocket 客户端
// 参数 cfg: WebSocket 配置
// 参数 symbolMaps: Symbol 映射表
// 参数 logger: 日志记录器
func NewClient(cfg *config.ExchangeWSConfig, symbolMaps map[string]*metadata.SymbolMap, logger *zap.Logger) *Client {
	return &Client{
		cfg:        cfg,
		symbolMaps: symbolMaps,
		logger:     logger.Named("okx"),
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

	// 设置请求头
	header := http.Header{}
	header.Set("Origin", "https://www.okx.com")
	header.Set("User-Agent", "latency-arbitrage-validator/1.0")

	// 建立连接
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, c.cfg.URL, header)
	if err != nil {
		return fmt.Errorf("连接 OKX WebSocket 失败: %w", err)
	}

	c.conn = conn
	c.backoff.Reset()
	c.logger.Info("OKX WebSocket 连接成功", zap.String("url", c.cfg.URL))

	return nil
}

// Subscribe 订阅交易对
// 订阅 books5 频道
func (c *Client) Subscribe() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("WebSocket 未连接")
	}

	// 构建订阅请求
	args := make([]SubscribeArg, 0, len(c.symbolMaps))
	for _, m := range c.symbolMaps {
		args = append(args, SubscribeArg{
			Channel: "books5",
			InstId:  m.OKXInstId,
		})
	}

	req := SubscribeRequest{
		Op:   "subscribe",
		Args: args,
	}

	// 发送订阅请求
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化订阅请求失败: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("发送订阅请求失败: %w", err)
	}

	c.logger.Info("OKX 订阅请求已发送", zap.Int("symbols", len(args)))
	return nil
}

// Run 启动客户端主循环
// 包含读取循环和心跳循环
func (c *Client) Run(ctx context.Context) {
	// 启动心跳 goroutine
	go c.heartbeatLoop(ctx)

	// 启动指标统计 goroutine
	go c.metricsLoop(ctx)

	// 读取循环
	c.readLoop(ctx)
}

// readLoop 读取循环
// 持续读取 WebSocket 消息并解析
func (c *Client) readLoop(ctx context.Context) {
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
			// 尝试重连
			c.reconnect(ctx)
			continue
		}

		// 读取消息
		_, data, err := conn.ReadMessage()
		if err != nil {
			c.logger.Warn("读取 OKX 消息失败", zap.Error(err))
			c.incrementReconnectCount()
			c.reconnect(ctx)
			continue
		}

		// 更新最后消息时间
		nowNs := timeutil.NowNano()
		atomic.StoreInt64(&c.lastMsgTime, nowNs)

		// 处理 pong 响应
		if IsPong(data) {
			atomic.StoreInt64(&c.lastPongRecvNs, nowNs)
			lastPing := atomic.LoadInt64(&c.lastPingSentNs)
			if lastPing > 0 {
				rttMs := (nowNs - lastPing) / 1_000_000
				c.metricsMu.Lock()
				c.metrics.WsRttMs = rttMs
				c.metricsMu.Unlock()
			}
			continue
		}

		// 处理订阅响应
		if IsSubscribeResponse(data) {
			c.logger.Debug("收到订阅响应", zap.ByteString("data", data))
			continue
		}

		// 解析 books5 消息
		events, err := c.parser.Parse(data)
		if err != nil {
			c.incrementParseErrorCount()
			c.maybeLogParseError(err, data)
			continue
		}

		// 发送事件到通道
		for _, event := range events {
			atomic.AddInt64(&c.updateCount, 1)
			select {
			case c.bookCh <- event:
			default:
				c.logger.Warn("OKX bookCh 已满，丢弃事件")
			}
		}
	}
}

// heartbeatLoop 心跳循环
// 每 25 秒发送 ping，期望 10 秒内收到 pong
func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(c.cfg.PingIntervalMs) * time.Millisecond)
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

			// 发送 ping（注意：gorilla/websocket 不允许并发多写者，这里用 connMu 串行化写入）
			pingTime := timeutil.NowNano()
			if err := conn.WriteMessage(websocket.TextMessage, []byte("ping")); err != nil {
				c.connMu.Unlock()
				c.logger.Warn("发送 OKX ping 失败", zap.Error(err))
				continue
			}
			atomic.StoreInt64(&c.lastPingSentNs, pingTime)
			c.connMu.Unlock()

			// 检查 pong 是否按期返回（允许与行情推送并行）
			lastPing := atomic.LoadInt64(&c.lastPingSentNs)
			lastPong := atomic.LoadInt64(&c.lastPongRecvNs)
			if lastPing > 0 && lastPong < lastPing {
				if timeutil.NowNano()-lastPing > int64(c.cfg.PongTimeoutMs)*1_000_000 {
					c.logger.Warn("OKX 心跳超时，触发重连")
					c.incrementReconnectCount()
					c.closeConn()
				}
			}
		}
	}
}

// metricsLoop 指标统计循环
// 每秒计算 QPS
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

			// 计算 QPS
			count := atomic.LoadInt64(&c.updateCount)
			qps := float64(count - lastCount)
			lastCount = count

			// 计算最后消息距今时间
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

// reconnect 重连
func (c *Client) reconnect(ctx context.Context) {
	c.closeConn()

	// 等待退避时间
	delay := c.backoff.Next()
	c.logger.Info("OKX 准备重连", zap.Duration("delay", delay))

	select {
	case <-ctx.Done():
		return
	case <-time.After(delay):
	}

	// 重新连接
	if err := c.Connect(ctx); err != nil {
		c.logger.Error("OKX 重连失败", zap.Error(err))
		return
	}

	// 重新订阅
	if err := c.Subscribe(); err != nil {
		c.logger.Error("OKX 重新订阅失败", zap.Error(err))
	}
}

// closeConn 关闭连接
func (c *Client) closeConn() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Close 关闭客户端
func (c *Client) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	c.closeConn()
	close(c.bookCh)
	close(c.errCh)
	c.logger.Info("OKX 客户端已关闭")
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

// incrementReconnectCount 增加重连计数
func (c *Client) incrementReconnectCount() {
	c.metricsMu.Lock()
	c.metrics.ReconnectCount++
	c.metricsMu.Unlock()
}

// incrementParseErrorCount 增加解析错误计数
func (c *Client) incrementParseErrorCount() {
	c.metricsMu.Lock()
	c.metrics.ParseErrorCount++
	c.metricsMu.Unlock()
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
	c.logger.Warn("解析 OKX 消息失败（采样）", zap.Error(err), zap.ByteString("data", sample))
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
