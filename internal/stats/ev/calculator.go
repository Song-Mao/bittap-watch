// Package ev 实现影子交易的期望值（EV）计算。
// EV = p × (R - f) + (1 - p) × (-L - f)
// p_required = (L + f) / (R + L)
package ev

import (
	"latency-arbitrage-validator/internal/core/model"
)

type tradeSample struct {
	win         bool
	grossPnLBps float64
	feeBps      float64
	netPnLBps   float64
	symbolCanon string
	exitReason  model.ExitReason
}

// EVStats EV 统计信息（滚动窗口）
type EVStats struct {
	// Count 样本数
	Count int64
	// WinCount 盈利样本数（净利>0）
	WinCount int64
	// LossCount 亏损样本数（净利<=0）
	LossCount int64

	// WinRate 胜率 p
	WinRate float64
	// AvgProfit 平均盈利 R（毛利，基点）
	AvgProfit float64
	// AvgLoss 平均亏损 L（毛亏损绝对值，基点）
	AvgLoss float64
	// FeeBps 平均手续费 f（基点，入场+出场）
	FeeBps float64

	// EV 期望值（基点）
	EV float64
	// PRequired 盈亏平衡胜率 p_required
	PRequired float64
}

// Calculator EV 计算器（滚动窗口）
// 仅用于研究/验证，输入来自影子成交结果（Position）。
type Calculator struct {
	// windowSize 滚动窗口大小
	windowSize int
	// buf 环形缓冲区
	buf []tradeSample
	// pos 写入位置
	pos int
	// full 是否已填满
	full bool

	// 维护滚动统计（O(1) 更新）
	count     int64
	winCount  int64
	lossCount int64
	sumWinR   float64
	sumLossL  float64
	sumFee    float64
}

// NewCalculator 创建 EV 计算器
// 参数 windowSize: 滚动窗口大小（建议 1000）
func NewCalculator(windowSize int) *Calculator {
	if windowSize <= 0 {
		windowSize = 1000
	}
	return &Calculator{
		windowSize: windowSize,
		buf:        make([]tradeSample, windowSize),
	}
}

// Add 添加一笔影子成交结果到滚动统计
func (c *Calculator) Add(pos *model.Position) {
	if pos == nil || !pos.Closed {
		return
	}

	s := tradeSample{
		win:         pos.NetPnLBps > 0,
		grossPnLBps: pos.GrossPnLBps,
		feeBps:      pos.FeeBps,
		netPnLBps:   pos.NetPnLBps,
		symbolCanon: pos.SymbolCanon,
		exitReason:  pos.ExitReason,
	}

	// 若环已满，移除旧样本对统计的贡献
	if c.full {
		old := c.buf[c.pos]
		c.count--
		if old.win {
			c.winCount--
			c.sumWinR -= old.grossPnLBps
		} else {
			c.lossCount--
			c.sumLossL -= abs(old.grossPnLBps)
		}
		c.sumFee -= old.feeBps
	}

	c.buf[c.pos] = s
	c.pos++
	if c.pos >= c.windowSize {
		c.pos = 0
		c.full = true
	}

	c.count++
	if s.win {
		c.winCount++
		c.sumWinR += s.grossPnLBps
	} else {
		c.lossCount++
		c.sumLossL += abs(s.grossPnLBps)
	}
	c.sumFee += s.feeBps
}

// Snapshot 获取当前 EV 统计快照
func (c *Calculator) Snapshot() *model.EVSnapshot {
	stats := c.Stats()
	return &model.EVSnapshot{
		WinRate:   stats.WinRate,
		AvgProfit: stats.AvgProfit,
		AvgLoss:   stats.AvgLoss,
		EV:        stats.EV,
		PRequired: stats.PRequired,
	}
}

// Stats 返回滚动窗口统计
func (c *Calculator) Stats() EVStats {
	out := EVStats{
		Count:     c.count,
		WinCount:  c.winCount,
		LossCount: c.lossCount,
	}
	if c.count <= 0 {
		return out
	}

	out.WinRate = float64(c.winCount) / float64(c.count)
	out.FeeBps = c.sumFee / float64(c.count)

	if c.winCount > 0 {
		out.AvgProfit = c.sumWinR / float64(c.winCount)
	}
	if c.lossCount > 0 {
		out.AvgLoss = c.sumLossL / float64(c.lossCount)
	}

	// EV = p × (R - f) + (1 - p) × (-L - f)
	p := out.WinRate
	R := out.AvgProfit
	L := out.AvgLoss
	f := out.FeeBps
	out.EV = p*(R-f) + (1-p)*(-L-f)

	// p_required = (L + f) / (R + L)
	den := R + L
	if den > 0 {
		out.PRequired = (L + f) / den
	} else {
		out.PRequired = 1
	}

	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
