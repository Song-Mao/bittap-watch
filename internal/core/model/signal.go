// Package model 定义验证器中使用的核心数据结构。
package model

import (
	"time"
)

// Signal 套利信号
// 当检测到 Leader 和 Follower 之间存在价差机会时生成
type Signal struct {
	// ID 信号唯一标识
	ID string
	// Leader 领先交易所标识: okx 或 binance
	Leader string
	// SymbolCanon 统一交易对标识，如 BTCUSDT
	SymbolCanon string
	// Side 交易方向: long 或 short
	// long: Leader.BestBid > Follower.BestAsk
	// short: Follower.BestBid > Leader.BestAsk
	Side Side
	// SpreadBps 入场价差（基点）
	// 计算公式:
	// long: (Leader.BestBid - Follower.BestAsk) / Follower.BestAsk * 10000
	// short: (Follower.BestBid - Leader.BestAsk) / Leader.BestAsk * 10000
	SpreadBps float64
	// LeaderBook 触发信号时的 Leader 订单簿快照
	LeaderBook *BookEvent
	// FollowerBook 触发信号时的 Follower 订单簿快照
	FollowerBook *BookEvent
	// DetectedAt 信号检测时间
	DetectedAt time.Time
	// DetectedAtNs 信号检测时间（纳秒时间戳）
	DetectedAtNs int64
	// RejectedByEV 是否因 EV 为负被拒绝
	RejectedByEV bool
	// FilterReason 过滤原因（若被过滤）
	FilterReason string
}

// IsLong 判断是否为多头信号
func (s *Signal) IsLong() bool {
	return s.Side == SideLong
}

// IsShort 判断是否为空头信号
func (s *Signal) IsShort() bool {
	return s.Side == SideShort
}

// Direction 获取方向系数
// 多头返回 1，空头返回 -1
func (s *Signal) Direction() float64 {
	if s.Side == SideLong {
		return 1
	}
	return -1
}
