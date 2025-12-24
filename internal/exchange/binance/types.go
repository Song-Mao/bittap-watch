// Package binance 定义 Binance 交易所消息类型。
package binance

// SubscribeRequest Binance WebSocket 订阅请求
// 订阅 depth5@100ms 行情流。
type SubscribeRequest struct {
	// Method 订阅方法: SUBSCRIBE
	Method string `json:"method"`
	// Params 订阅参数列表，如 "btcusdt@depth5@100ms"
	Params []string `json:"params"`
	// ID 请求 ID
	ID int64 `json:"id"`
}

// SubscribeResponse Binance WebSocket 订阅响应
// 通常形如 {"result":null,"id":1}。
type SubscribeResponse struct {
	// Result 结果（成功为 null）
	Result any `json:"result"`
	// ID 请求 ID
	ID int64 `json:"id"`
}

// DepthUpdate Binance 深度推送消息（depthUpdate）
// 字段映射：
// - e: 事件类型（depthUpdate）
// - E: 事件时间（毫秒） -> BookEvent.ExchTsUnixMs
// - s: Symbol（如 BTCUSDT） -> BookEvent.SymbolCanon（与 Canon 一致）
// - b: bids [[price, qty], ...]（字符串）
// - a: asks [[price, qty], ...]（字符串）
type DepthUpdate struct {
	// EventType 事件类型: depthUpdate
	EventType string `json:"e"`
	// EventTimeMs 事件时间（毫秒）
	EventTimeMs int64 `json:"E"`
	// Symbol 交易对（大写）
	Symbol string `json:"s"`
	// Bids 买盘档位（价格、数量）
	Bids [][]string `json:"b"`
	// Asks 卖盘档位（价格、数量）
	Asks [][]string `json:"a"`
}

// ConnectionMetrics 连接质量指标
type ConnectionMetrics struct {
	// ReconnectCount 重连次数
	ReconnectCount int64
	// ParseErrorCount 解析错误次数
	ParseErrorCount int64
	// UpdatesPerSec 每秒更新次数
	UpdatesPerSec float64
	// LastMessageAgeMs 最后消息距今时间（毫秒）
	LastMessageAgeMs int64
}
