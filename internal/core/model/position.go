// Package model 定义验证器中使用的核心数据结构。
package model

import (
	"time"
)

// ExitReason 退出原因
type ExitReason string

const (
	// ExitTP 止盈退出
	// 当价差收敛到 (1 - r_tp) × 入场价差 时触发
	ExitTP ExitReason = "tp"
	// ExitSL 止损退出
	// 当价差发散到 (1 + r_sl) × 入场价差 时触发
	ExitSL ExitReason = "sl"
	// ExitTimeout 超时退出
	// 当持仓时间超过 max_hold_ms 时触发
	ExitTimeout ExitReason = "timeout"
)

// Position 影子仓位
// 用于模拟交易，不进行真实下单
type Position struct {
	// ID 仓位唯一标识
	ID string
	// Leader 领先交易所标识: okx 或 binance
	Leader string
	// SymbolCanon 统一交易对标识
	SymbolCanon string
	// Side 交易方向: long 或 short
	Side Side
	// EntryPx 入场价格
	// long: 使用 Follower.BestAsk
	// short: 使用 Follower.BestBid
	EntryPx float64
	// EntrySpread 入场时的价差（基点）
	EntrySpread float64
	// EntryTime 入场时间
	EntryTime time.Time
	// EntryTimeNs 入场时间（纳秒时间戳）
	EntryTimeNs int64
	// ExitPx 出场价格
	// long: 使用 Follower.BestBid
	// short: 使用 Follower.BestAsk
	ExitPx float64
	// ExitTime 出场时间
	ExitTime time.Time
	// ExitTimeNs 出场时间（纳秒时间戳）
	ExitTimeNs int64
	// ExitReason 退出原因: tp, sl, timeout
	ExitReason ExitReason
	// GrossPnLBps 毛利（基点）
	// 计算公式: (exit_px - entry_px) / entry_px × 10000 × direction
	GrossPnLBps float64
	// FeeBps 手续费（基点）
	// 计算公式: 2 × effective_fee × 10000（入场 + 出场）
	FeeBps float64
	// NetPnLBps 净利（基点）
	// 计算公式: gross_pnl_bps - fee_bps
	NetPnLBps float64
	// Closed 是否已平仓
	Closed bool
}

// IsLong 判断是否为多头仓位
func (p *Position) IsLong() bool {
	return p.Side == SideLong
}

// IsShort 判断是否为空头仓位
func (p *Position) IsShort() bool {
	return p.Side == SideShort
}

// Direction 获取方向系数
// 多头返回 1，空头返回 -1
func (p *Position) Direction() float64 {
	if p.Side == SideLong {
		return 1
	}
	return -1
}

// HoldDuration 获取持仓时长
func (p *Position) HoldDuration() time.Duration {
	if p.Closed {
		return p.ExitTime.Sub(p.EntryTime)
	}
	return time.Since(p.EntryTime)
}

// HoldDurationMs 获取持仓时长（毫秒）
func (p *Position) HoldDurationMs() int64 {
	return p.HoldDuration().Milliseconds()
}

// IsWin 判断是否盈利
func (p *Position) IsWin() bool {
	return p.NetPnLBps > 0
}

// IsLoss 判断是否亏损
func (p *Position) IsLoss() bool {
	return p.NetPnLBps < 0
}

// PaperTrade 影子成交输出结构
// 用于 JSONL 文件输出，包含所有必需字段
type PaperTrade struct {
	// Leader 领先交易所
	Leader string `json:"leader"`
	// SymbolCanon 统一交易对
	SymbolCanon string `json:"symbol_canon"`
	// Side 交易方向
	Side string `json:"side"`
	// TEntryNs 入场时间（纳秒）
	TEntryNs int64 `json:"t_entry_ns"`
	// TExitNs 出场时间（纳秒）
	TExitNs int64 `json:"t_exit_ns"`
	// EntryPx 入场价格
	EntryPx float64 `json:"entry_px"`
	// ExitPx 出场价格
	ExitPx float64 `json:"exit_px"`
	// GrossPnLBps 毛利（基点）
	GrossPnLBps float64 `json:"gross_pnl_bps"`
	// FeeBps 手续费（基点）
	FeeBps float64 `json:"fee_bps"`
	// NetPnLBps 净利（基点）
	NetPnLBps float64 `json:"net_pnl_bps"`
	// ExitReason 退出原因
	ExitReason string `json:"exit_reason"`
	// EVSnapshot EV 快照（可选）
	EVSnapshot *EVSnapshot `json:"ev_snapshot,omitempty"`
}

// EVSnapshot EV 统计快照
type EVSnapshot struct {
	// WinRate 胜率
	WinRate float64 `json:"win_rate"`
	// AvgProfit 平均盈利（基点）
	AvgProfit float64 `json:"avg_profit"`
	// AvgLoss 平均亏损（基点）
	AvgLoss float64 `json:"avg_loss"`
	// EV 期望值
	EV float64 `json:"ev"`
	// PRequired 盈亏平衡胜率
	PRequired float64 `json:"p_required"`
}

// ToPaperTrade 将 Position 转换为 PaperTrade 输出格式
func (p *Position) ToPaperTrade(evSnapshot *EVSnapshot) *PaperTrade {
	return &PaperTrade{
		Leader:      p.Leader,
		SymbolCanon: p.SymbolCanon,
		Side:        string(p.Side),
		TEntryNs:    p.EntryTimeNs,
		TExitNs:     p.ExitTimeNs,
		EntryPx:     p.EntryPx,
		ExitPx:      p.ExitPx,
		GrossPnLBps: p.GrossPnLBps,
		FeeBps:      p.FeeBps,
		NetPnLBps:   p.NetPnLBps,
		ExitReason:  string(p.ExitReason),
		EVSnapshot:  evSnapshot,
	}
}
