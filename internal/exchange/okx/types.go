// Package okx 定义 OKX 交易所消息类型。
package okx

// SubscribeRequest OKX 订阅请求
// 用于订阅 books5 频道
type SubscribeRequest struct {
	// Op 操作类型: subscribe, unsubscribe
	Op string `json:"op"`
	// Args 订阅参数列表
	Args []SubscribeArg `json:"args"`
}

// SubscribeArg 订阅参数
type SubscribeArg struct {
	// Channel 频道名称: books5
	Channel string `json:"channel"`
	// InstId 合约 ID: BTC-USDT-SWAP
	InstId string `json:"instId"`
}

// SubscribeResponse OKX 订阅响应
type SubscribeResponse struct {
	// Event 事件类型: subscribe, error
	Event string `json:"event"`
	// Arg 订阅参数
	Arg *SubscribeArg `json:"arg,omitempty"`
	// Code 错误码
	Code string `json:"code,omitempty"`
	// Msg 错误消息
	Msg string `json:"msg,omitempty"`
}

// Books5Message OKX books5 频道消息
// 包含 5 档深度数据
type Books5Message struct {
	// Arg 订阅参数
	Arg SubscribeArg `json:"arg"`
	// Action 动作类型: snapshot, update
	Action string `json:"action"`
	// Data 深度数据列表
	Data []Books5Data `json:"data"`
}

// Books5Data OKX books5 深度数据
// 字段映射:
// - bids: 买盘深度 [[价格, 数量, 废弃, 订单数], ...]
// - asks: 卖盘深度 [[价格, 数量, 废弃, 订单数], ...]
// - ts: 交易所时间戳（毫秒）
// - seqId: 序列号
type Books5Data struct {
	// Bids 买盘深度: [[价格, 数量, 废弃, 订单数], ...]
	Bids [][]string `json:"bids"`
	// Asks 卖盘深度: [[价格, 数量, 废弃, 订单数], ...]
	Asks [][]string `json:"asks"`
	// Ts 交易所时间戳（毫秒字符串）
	Ts string `json:"ts"`
	// SeqId 序列号
	SeqId int64 `json:"seqId"`
	// InstId 合约 ID
	InstId string `json:"instId"`
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
	// WsRttMs WebSocket RTT（毫秒）
	WsRttMs int64
}
