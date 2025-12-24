// Package paper 影子成交执行器测试
package paper

import (
	"testing"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
)

func TestExecutor_TakeProfit_Long(t *testing.T) {
	exec := NewExecutor(model.ExchangeOKX, config.PaperConfig{
		TPRatio:     0.5,
		SLRatio:     1.0,
		MaxHoldMs:   60000,
		SlippageBps: 0,
	}, config.FeeDetail{})

	sig := &model.Signal{
		Leader:       model.ExchangeOKX,
		SymbolCanon:  "BTCUSDT",
		Side:         model.SideLong,
		SpreadBps:    100,
		DetectedAtNs: 1_000_000_000,
		LeaderBook: &model.BookEvent{
			Exchange:    model.ExchangeOKX,
			SymbolCanon: "BTCUSDT",
			BestBidPx:   100.00,
			BestAskPx:   100.10,
		},
		FollowerBook: &model.BookEvent{
			Exchange:    model.ExchangeBittap,
			SymbolCanon: "BTCUSDT",
			BestBidPx:   99.80,
			BestAskPx:   99.90,
		},
	}

	_, opened, err := exec.TryOpen(sig)
	if err != nil || !opened {
		t.Fatalf("TryOpen failed: opened=%v err=%v", opened, err)
	}

	// 价差收敛：long_spread = (100 - 99.99)/99.99*10000 ≈ 1 bps < 50 bps
	leaderNow := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.10}
	followerNow := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 100.01, BestAskPx: 99.99}
	closed := exec.Evaluate(1_200_000_000, leaderNow, followerNow)
	if closed == nil {
		t.Fatalf("应触发止盈平仓")
	}
	if closed.ExitReason != model.ExitTP {
		t.Fatalf("ExitReason=%s, want tp", closed.ExitReason)
	}
	if !closed.Closed {
		t.Fatalf("Closed=false, want true")
	}
}

func TestExecutor_StopLoss_Long(t *testing.T) {
	exec := NewExecutor(model.ExchangeOKX, config.PaperConfig{
		TPRatio:     0.5,
		SLRatio:     0.5, // 1.5x
		MaxHoldMs:   60000,
		SlippageBps: 0,
	}, config.FeeDetail{})

	sig := &model.Signal{
		Leader:       model.ExchangeOKX,
		SymbolCanon:  "BTCUSDT",
		Side:         model.SideLong,
		SpreadBps:    100,
		DetectedAtNs: 1_000_000_000,
		LeaderBook:   &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.10},
		FollowerBook: &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.80, BestAskPx: 99.90},
	}

	_, opened, err := exec.TryOpen(sig)
	if err != nil || !opened {
		t.Fatalf("TryOpen failed: opened=%v err=%v", opened, err)
	}

	// 价差发散：FollowerAsk 大幅下降，long_spread 变大
	leaderNow := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.10}
	followerNow := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 90.00, BestAskPx: 90.01}
	closed := exec.Evaluate(1_200_000_000, leaderNow, followerNow)
	if closed == nil {
		t.Fatalf("应触发止损平仓")
	}
	if closed.ExitReason != model.ExitSL {
		t.Fatalf("ExitReason=%s, want sl", closed.ExitReason)
	}
}

func TestExecutor_Timeout(t *testing.T) {
	exec := NewExecutor(model.ExchangeOKX, config.PaperConfig{
		TPRatio:     0.0,
		SLRatio:     0.0,
		MaxHoldMs:   10,
		SlippageBps: 0,
	}, config.FeeDetail{})

	sig := &model.Signal{
		Leader:       model.ExchangeOKX,
		SymbolCanon:  "BTCUSDT",
		Side:         model.SideLong,
		SpreadBps:    100,
		DetectedAtNs: 1_000_000_000,
		LeaderBook:   &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.10},
		FollowerBook: &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.80, BestAskPx: 99.90},
	}

	_, opened, err := exec.TryOpen(sig)
	if err != nil || !opened {
		t.Fatalf("TryOpen failed: opened=%v err=%v", opened, err)
	}

	leaderNow := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.10}
	followerNow := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.80, BestAskPx: 99.90}
	closed := exec.Evaluate(1_020_000_000, leaderNow, followerNow) // +20ms
	if closed == nil || closed.ExitReason != model.ExitTimeout {
		t.Fatalf("应触发超时平仓")
	}
}
