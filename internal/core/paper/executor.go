// Package paper 实现影子/模拟成交的仓位管理。
// 重要：仅用于研究，严禁真实下单。
package paper

import (
	"fmt"
	"math"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/util/timeutil"
)

// Executor 影子成交执行器（单 Leader 链路）
// 重要：仅用于研究/验证，严禁真实下单。
type Executor struct {
	// leader 领先交易所: okx 或 binance
	leader string
	// cfg 影子成交配置
	cfg config.PaperConfig
	// fee 手续费配置（用于计算有效 taker fee）
	fee config.FeeDetail

	// positions 当前持仓（按交易对）
	positions map[string]*model.Position
}

// NewExecutor 创建影子成交执行器
// 参数 leader: okx 或 binance
// 参数 cfg: 影子成交配置
// 参数 fee: Bittap 手续费配置
func NewExecutor(leader string, cfg config.PaperConfig, fee config.FeeDetail) *Executor {
	return &Executor{
		leader:    leader,
		cfg:       cfg,
		fee:       fee,
		positions: make(map[string]*model.Position),
	}
}

// TryOpen 尝试根据信号开仓
// 若该交易对已有未平仓仓位，则返回 (nil, false, nil)。
func (e *Executor) TryOpen(sig *model.Signal) (*model.Position, bool, error) {
	if sig == nil || sig.Leader != e.leader || sig.SymbolCanon == "" {
		return nil, false, nil
	}
	if sig.FollowerBook == nil || sig.LeaderBook == nil {
		return nil, false, fmt.Errorf("信号缺少订单簿快照")
	}
	if sig.FollowerBook.Exchange != model.ExchangeBittap {
		return nil, false, fmt.Errorf("Follower 必须为 bittap")
	}

	if pos := e.positions[sig.SymbolCanon]; pos != nil && !pos.Closed {
		return nil, false, nil
	}

	entryPx, err := e.entryPx(sig.Side, sig.FollowerBook)
	if err != nil {
		return nil, false, err
	}

	pos := &model.Position{
		ID:          fmt.Sprintf("paper-%s-%s-%d", e.leader, sig.SymbolCanon, sig.DetectedAtNs),
		Leader:      e.leader,
		SymbolCanon: sig.SymbolCanon,
		Side:        sig.Side,
		EntryPx:     entryPx,
		EntrySpread: sig.SpreadBps,
		EntryTime:   timeutil.NanoToTime(sig.DetectedAtNs),
		EntryTimeNs: sig.DetectedAtNs,
		Closed:      false,
	}

	// 手续费采用 taker，有效费率 = raw_fee × (1 - rebate_rate)
	// round-trip fee_bps = 2 × effective_fee × 10000
	effectiveFee := e.fee.EffectiveTakerFee()
	pos.FeeBps = 2 * effectiveFee * 10000

	e.positions[sig.SymbolCanon] = pos
	return pos, true, nil
}

// Evaluate 评估持仓是否触发退出条件
// 返回：若平仓则返回已平仓的 Position；否则返回 nil。
func (e *Executor) Evaluate(nowNs int64, leaderBook, followerBook *model.BookEvent) *model.Position {
	if leaderBook == nil || followerBook == nil {
		return nil
	}
	if leaderBook.Exchange != e.leader || followerBook.Exchange != model.ExchangeBittap {
		return nil
	}
	if leaderBook.SymbolCanon == "" || followerBook.SymbolCanon == "" || leaderBook.SymbolCanon != followerBook.SymbolCanon {
		return nil
	}

	pos := e.positions[leaderBook.SymbolCanon]
	if pos == nil || pos.Closed {
		return nil
	}

	curSpread, ok := currentSpreadBps(pos.Side, leaderBook, followerBook)
	if !ok {
		return nil
	}

	entryAbs := math.Abs(pos.EntrySpread)
	curAbs := math.Abs(curSpread)

	// TP：|current_spread| ≤ (1 - r_tp) × |entry_spread|
	if e.cfg.TPRatio > 0 && entryAbs > 0 && curAbs <= (1.0-e.cfg.TPRatio)*entryAbs {
		return e.close(nowNs, pos, followerBook, model.ExitTP)
	}
	// SL：|current_spread| ≥ (1 + r_sl) × |entry_spread|
	if e.cfg.SLRatio > 0 && entryAbs > 0 && curAbs >= (1.0+e.cfg.SLRatio)*entryAbs {
		return e.close(nowNs, pos, followerBook, model.ExitSL)
	}
	// Timeout：持仓超过 max_hold_ms
	if e.cfg.MaxHoldMs > 0 && (nowNs-pos.EntryTimeNs) > int64(e.cfg.MaxHoldMs)*1_000_000 {
		return e.close(nowNs, pos, followerBook, model.ExitTimeout)
	}

	return nil
}

func (e *Executor) close(nowNs int64, pos *model.Position, followerBook *model.BookEvent, reason model.ExitReason) *model.Position {
	exitPx, err := e.exitPx(pos.Side, followerBook)
	if err != nil {
		return nil
	}

	pos.ExitPx = exitPx
	pos.ExitTimeNs = nowNs
	pos.ExitTime = timeutil.NanoToTime(nowNs)
	pos.ExitReason = reason
	pos.Closed = true

	// gross_pnl_bps = (exit_px - entry_px) / entry_px × 10000 × direction
	pos.GrossPnLBps = (pos.ExitPx - pos.EntryPx) / pos.EntryPx * 10000 * pos.Direction()
	// net_pnl_bps = gross_pnl_bps - fee_bps
	pos.NetPnLBps = pos.GrossPnLBps - pos.FeeBps

	return pos
}

func (e *Executor) entryPx(side model.Side, followerBook *model.BookEvent) (float64, error) {
	if followerBook == nil {
		return 0, fmt.Errorf("follower book 为空")
	}
	slip := e.cfg.SlippageBps / 10000
	switch side {
	case model.SideLong:
		if followerBook.BestAskPx <= 0 {
			return 0, fmt.Errorf("BestAskPx 无效")
		}
		return followerBook.BestAskPx * (1 + slip), nil
	case model.SideShort:
		if followerBook.BestBidPx <= 0 {
			return 0, fmt.Errorf("BestBidPx 无效")
		}
		return followerBook.BestBidPx * (1 - slip), nil
	default:
		return 0, fmt.Errorf("未知 side: %s", side)
	}
}

func (e *Executor) exitPx(side model.Side, followerBook *model.BookEvent) (float64, error) {
	if followerBook == nil {
		return 0, fmt.Errorf("follower book 为空")
	}
	slip := e.cfg.SlippageBps / 10000
	switch side {
	case model.SideLong:
		if followerBook.BestBidPx <= 0 {
			return 0, fmt.Errorf("BestBidPx 无效")
		}
		return followerBook.BestBidPx * (1 - slip), nil
	case model.SideShort:
		if followerBook.BestAskPx <= 0 {
			return 0, fmt.Errorf("BestAskPx 无效")
		}
		return followerBook.BestAskPx * (1 + slip), nil
	default:
		return 0, fmt.Errorf("未知 side: %s", side)
	}
}

func currentSpreadBps(side model.Side, leaderBook, followerBook *model.BookEvent) (float64, bool) {
	switch side {
	case model.SideLong:
		if followerBook.BestAskPx <= 0 || leaderBook.BestBidPx <= 0 {
			return 0, false
		}
		return (leaderBook.BestBidPx - followerBook.BestAskPx) / followerBook.BestAskPx * 10000, true
	case model.SideShort:
		if leaderBook.BestAskPx <= 0 || followerBook.BestBidPx <= 0 {
			return 0, false
		}
		return (followerBook.BestBidPx - leaderBook.BestAskPx) / leaderBook.BestAskPx * 10000, true
	default:
		return 0, false
	}
}
