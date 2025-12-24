// Package bittap 定义 Bittap 交易所消息类型。
package bittap

// SubscribeRequest Bittap WebSocket 订阅请求
// 订阅频道格式：f_depth30@{symbol}_{tick}。
type SubscribeRequest struct {
	// Method 订阅方法: SUBSCRIBE
	Method string `json:"method"`
	// Params 订阅参数列表，如 "f_depth30@BTC-USDT-M_0.1"
	Params []string `json:"params"`
	// ID 请求 ID（Bittap 文档示例为字符串）
	ID string `json:"id"`
}

// PingRequest Bittap WebSocket 心跳请求
type PingRequest struct {
	// ID 请求 ID
	ID string `json:"id"`
	// Method 方法: PING
	Method string `json:"method"`
}

// PongResponse Bittap WebSocket 心跳响应（可能的形式之一）
type PongResponse struct {
	// Result 结果: "PONG"（也可能为 null）
	Result *string `json:"result"`
	// Method 方法: "PONG"（另一种可能形式）
	Method string `json:"method"`
}

// DepthMessage Bittap 深度推送消息（f_depth30）
// 字段映射：
// - e: "f_depth30"
// - s: 交易对（如 BTC-USDT-M）
// - i: 档位（如 0.1）
// - lastUpdateId: 序列号 -> BookEvent.Seq
// - bids/asks: 二维数组字符串（价格、数量）
type DepthMessage struct {
	// Event 事件类型: f_depth30
	Event string `json:"e"`
	// Symbol 交易对，如 BTC-USDT 或 BTC-USDT-M
	Symbol string `json:"s"`
	// Tick 档位字符串，如 "0.1"
	Tick string `json:"i"`
	// LastUpdateID 序列号
	LastUpdateID int64 `json:"lastUpdateId"`
	// Bids 买盘档位（高->低）
	Bids [][]string `json:"bids"`
	// Asks 卖盘档位（低->高）
	Asks [][]string `json:"asks"`
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
