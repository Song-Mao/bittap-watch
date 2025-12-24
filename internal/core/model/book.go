// Package model 定义验证器中使用的核心数据结构。
// 包含订单簿事件、信号、仓位等核心类型。
package model

import (
	"time"
)

// Exchange 交易所标识常量
const (
	// ExchangeOKX OKX 交易所
	ExchangeOKX = "okx"
	// ExchangeBinance Binance 交易所
	ExchangeBinance = "binance"
	// ExchangeBittap Bittap 交易所（Follower）
	ExchangeBittap = "bittap"
)

// Side 交易方向
type Side string

const (
	// SideLong 多头方向
	// 当 Leader.BestBid > Follower.BestAsk 时触发
	SideLong Side = "long"
	// SideShort 空头方向
	// 当 Follower.BestBid > Leader.BestAsk 时触发
	SideShort Side = "short"
)

// Level 订单簿深度档位
// 表示某一价格档位的价格和数量
type Level struct {
	// Price 价格
	Price float64
	// Qty 数量
	Qty float64
}

// BookEvent 统一订单簿事件结构
// 用于归一化三家交易所的订单簿数据，便于跨交易所分析
type BookEvent struct {
	// Exchange 交易所标识: okx, binance, bittap
	Exchange string
	// SymbolCanon 统一交易对标识，如 BTCUSDT
	SymbolCanon string
	// BestBidPx 最优买价（买一价）
	BestBidPx float64
	// BestBidQty 最优买量（买一量）
	BestBidQty float64
	// BestAskPx 最优卖价（卖一价）
	BestAskPx float64
	// BestAskQty 最优卖量（卖一量）
	BestAskQty float64
	// Levels 深度档位列表（Top 5）
	// 包含买卖双方的深度信息
	Levels []Level
	// ArrivedAtUnixNs 本机收到消息的时间戳（纳秒）
	// 用于计算 lead-lag 延迟，是延迟统计的主基准
	ArrivedAtUnixNs int64
	// ExchTsUnixMs 交易所事件时间戳（毫秒）
	// OKX: ts 字段
	// Binance: E 字段
	// Bittap: 无此字段，设为 0
	ExchTsUnixMs int64
	// Seq 序列号
	// OKX: seqId 字段
	// Bittap: lastUpdateId 字段
	// Binance: 无此字段，设为 0
	Seq int64
}

// IsValid 检查订单簿事件是否有效
// 有效条件: 买卖价格都大于 0，且买价 < 卖价
func (b *BookEvent) IsValid() bool {
	return b.BestBidPx > 0 && b.BestAskPx > 0 && b.BestBidPx < b.BestAskPx
}

// MidPrice 计算中间价
// 公式: (BestBidPx + BestAskPx) / 2
func (b *BookEvent) MidPrice() float64 {
	return (b.BestBidPx + b.BestAskPx) / 2
}

// Spread 计算买卖价差
// 公式: BestAskPx - BestBidPx
func (b *BookEvent) Spread() float64 {
	return b.BestAskPx - b.BestBidPx
}

// SpreadBps 计算买卖价差（基点）
// 公式: (BestAskPx - BestBidPx) / MidPrice * 10000
func (b *BookEvent) SpreadBps() float64 {
	mid := b.MidPrice()
	if mid == 0 {
		return 0
	}
	return (b.BestAskPx - b.BestBidPx) / mid * 10000
}

// Top5DepthUSD 计算前 5 档深度的 USD 价值
// 用于深度过滤器
func (b *BookEvent) Top5DepthUSD() float64 {
	var total float64
	for i, level := range b.Levels {
		if i >= 5 {
			break
		}
		total += level.Price * level.Qty
	}
	return total
}

// ArrivedAt 获取到达时间的 time.Time 表示
func (b *BookEvent) ArrivedAt() time.Time {
	return time.Unix(0, b.ArrivedAtUnixNs)
}

// ExchTs 获取交易所时间的 time.Time 表示
// 若 ExchTsUnixMs 为 0，返回零值
func (b *BookEvent) ExchTs() time.Time {
	if b.ExchTsUnixMs == 0 {
		return time.Time{}
	}
	return time.UnixMilli(b.ExchTsUnixMs)
}

// Clone 创建 BookEvent 的深拷贝
func (b *BookEvent) Clone() *BookEvent {
	clone := *b
	if b.Levels != nil {
		clone.Levels = make([]Level, len(b.Levels))
		copy(clone.Levels, b.Levels)
	}
	return &clone
}
