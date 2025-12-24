// Package signal 信号引擎测试
package signal

import (
	"testing"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
)

func TestEngine_PersistFilter_Long(t *testing.T) {
	e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
		ThetaEntryBps: 10,
		PersistMs:     100,
		MinDepthUSD:   0,
		CooldownMs:    0,
	})

	leader := &model.BookEvent{
		Exchange:    model.ExchangeOKX,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   100.00,
		BestAskPx:   100.01,
		Levels:      []model.Level{{Price: 100.00, Qty: 100}},
	}
	follower := &model.BookEvent{
		Exchange:    model.ExchangeBittap,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   99.80,
		BestAskPx:   99.90,
		Levels:      []model.Level{{Price: 99.90, Qty: 100}},
	}

	now := int64(1_000_000_000)
	if sig := e.Evaluate(now, leader, follower); sig != nil {
		t.Fatalf("首次触发不应立刻产生信号")
	}

	// 未达到 persist 窗口，不应出信号
	if sig := e.Evaluate(now+50*1_000_000, leader, follower); sig != nil {
		t.Fatalf("persist 未到期不应产生信号")
	}

	// 到期，产生信号
	sig := e.Evaluate(now+110*1_000_000, leader, follower)
	if sig == nil {
		t.Fatalf("persist 到期应产生信号")
	}
	if sig.Side != model.SideLong {
		t.Fatalf("Side=%s, want long", sig.Side)
	}
	if sig.Leader != model.ExchangeOKX {
		t.Fatalf("Leader=%s, want okx", sig.Leader)
	}

	// 条件持续成立，不应重复出信号（需等待条件失效后重新武装）
	if sig2 := e.Evaluate(now+200*1_000_000, leader, follower); sig2 != nil {
		t.Fatalf("不应重复产生信号")
	}

	// 条件失效后重置
	follower.BestAskPx = 100.50
	if sig3 := e.Evaluate(now+300*1_000_000, leader, follower); sig3 != nil {
		t.Fatalf("条件失效不应产生信号")
	}
}

func TestEngine_PersistFilter_Short(t *testing.T) {
	e := NewEngine(model.ExchangeBinance, config.StrategyConfig{
		ThetaEntryBps: 10,
		PersistMs:     100,
		MinDepthUSD:   0,
		CooldownMs:    0,
	})

	leader := &model.BookEvent{
		Exchange:    model.ExchangeBinance,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   100.00,
		BestAskPx:   100.10,
		Levels:      []model.Level{{Price: 100.10, Qty: 100}},
	}
	follower := &model.BookEvent{
		Exchange:    model.ExchangeBittap,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   100.50,
		BestAskPx:   100.60,
		Levels:      []model.Level{{Price: 100.50, Qty: 100}},
	}

	now := int64(1_000_000_000)
	_ = e.Evaluate(now, leader, follower)

	sig := e.Evaluate(now+120*1_000_000, leader, follower)
	if sig == nil {
		t.Fatalf("persist 到期应产生信号")
	}
	if sig.Side != model.SideShort {
		t.Fatalf("Side=%s, want short", sig.Side)
	}
}

func TestEngine_DepthFilter(t *testing.T) {
	e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
		ThetaEntryBps: 10,
		PersistMs:     0,
		MinDepthUSD:   1_000_000, // 极高深度阈值
		CooldownMs:    0,
	})

	leader := &model.BookEvent{
		Exchange:    model.ExchangeOKX,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   100.00,
		BestAskPx:   100.01,
		Levels:      []model.Level{{Price: 100.00, Qty: 1}},
	}
	follower := &model.BookEvent{
		Exchange:    model.ExchangeBittap,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   99.80,
		BestAskPx:   99.90,
		Levels:      []model.Level{{Price: 99.90, Qty: 1}},
	}

	if sig := e.Evaluate(1_000_000_000, leader, follower); sig != nil {
		t.Fatalf("深度不足不应产生信号")
	}
}

func TestEngine_CooldownAfterStopLoss(t *testing.T) {
	e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
		ThetaEntryBps: 10,
		PersistMs:     0,
		MinDepthUSD:   0,
		CooldownMs:    3000,
	})

	leader := &model.BookEvent{
		Exchange:    model.ExchangeOKX,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   100.00,
		BestAskPx:   100.01,
		Levels:      []model.Level{{Price: 100.00, Qty: 100}},
	}
	follower := &model.BookEvent{
		Exchange:    model.ExchangeBittap,
		SymbolCanon: "BTCUSDT",
		BestBidPx:   99.80,
		BestAskPx:   99.90,
		Levels:      []model.Level{{Price: 99.90, Qty: 100}},
	}

	now := int64(1_000_000_000)
	e.NotifyStopLoss("BTCUSDT", now)
	if sig := e.Evaluate(now+1_000_000, leader, follower); sig != nil {
		t.Fatalf("冷却期内不应产生信号")
	}
	if sig := e.Evaluate(now+3_100*1_000_000, leader, follower); sig == nil {
		t.Fatalf("冷却结束后应允许产生信号")
	}
}
