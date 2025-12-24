// Package signal 信号引擎属性测试
package signal

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
)

// **Feature: latency-arbitrage-validator, Property 11: Signal Detection Correctness**
// **Validates: Requirements 5.1, 5.2**

func TestEngine_SignalDetection_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("满足 long 条件应产生 long 信号（persist=0）", prop.ForAll(
		func(thetaBps float64, followerAsk float64, spreadBps float64) bool {
			if thetaBps < 0.1 {
				thetaBps = 0.1
			}
			if followerAsk <= 0 {
				followerAsk = 100
			}
			if spreadBps <= thetaBps {
				spreadBps = thetaBps + 1
			}

			leaderBid := followerAsk * (1 + spreadBps/10000)

			e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
				ThetaEntryBps:    thetaBps,
				PersistMs:        0,
				MinDepthUSD:      0,
				VolFilterEnabled: false,
				CooldownMs:       0,
			})

			leader := &model.BookEvent{
				Exchange:    model.ExchangeOKX,
				SymbolCanon: "BTCUSDT",
				BestBidPx:   leaderBid,
				BestAskPx:   leaderBid + 0.01,
				Levels:      []model.Level{{Price: leaderBid, Qty: 10}},
			}
			follower := &model.BookEvent{
				Exchange:    model.ExchangeBittap,
				SymbolCanon: "BTCUSDT",
				BestBidPx:   followerAsk - 0.01,
				BestAskPx:   followerAsk,
				Levels:      []model.Level{{Price: followerAsk, Qty: 10}},
			}

			sig := e.Evaluate(1_000_000_000, leader, follower)
			return sig != nil && sig.Side == model.SideLong && sig.Leader == model.ExchangeOKX
		},
		gen.Float64Range(0.1, 200),
		gen.Float64Range(1, 200000),
		gen.Float64Range(0.2, 500),
	))

	properties.Property("满足 short 条件应产生 short 信号（persist=0）", prop.ForAll(
		func(thetaBps float64, leaderAsk float64, spreadBps float64) bool {
			if thetaBps < 0.1 {
				thetaBps = 0.1
			}
			if leaderAsk <= 0 {
				leaderAsk = 100
			}
			if spreadBps <= thetaBps {
				spreadBps = thetaBps + 1
			}

			followerBid := leaderAsk * (1 + spreadBps/10000)

			e := NewEngine(model.ExchangeBinance, config.StrategyConfig{
				ThetaEntryBps:    thetaBps,
				PersistMs:        0,
				MinDepthUSD:      0,
				VolFilterEnabled: false,
				CooldownMs:       0,
			})

			leader := &model.BookEvent{
				Exchange:    model.ExchangeBinance,
				SymbolCanon: "BTCUSDT",
				BestBidPx:   leaderAsk - 0.01,
				BestAskPx:   leaderAsk,
				Levels:      []model.Level{{Price: leaderAsk, Qty: 10}},
			}
			follower := &model.BookEvent{
				Exchange:    model.ExchangeBittap,
				SymbolCanon: "BTCUSDT",
				BestBidPx:   followerBid,
				BestAskPx:   followerBid + 0.01,
				Levels:      []model.Level{{Price: followerBid, Qty: 10}},
			}

			sig := e.Evaluate(1_000_000_000, leader, follower)
			return sig != nil && sig.Side == model.SideShort && sig.Leader == model.ExchangeBinance
		},
		gen.Float64Range(0.1, 200),
		gen.Float64Range(1, 200000),
		gen.Float64Range(0.2, 500),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 12: Persistence Filter Correctness**
// **Validates: Requirements 5.3**

func TestEngine_PersistenceFilter_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 80
	properties := gopter.NewProperties(parameters)

	properties.Property("persist 未到期不出信号，到期后出信号", prop.ForAll(
		func(persistMs int) bool {
			if persistMs < 1 {
				persistMs = 1
			}

			e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
				ThetaEntryBps:    10,
				PersistMs:        persistMs,
				MinDepthUSD:      0,
				VolFilterEnabled: false,
				CooldownMs:       0,
			})

			leader := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.01, Levels: []model.Level{{Price: 100, Qty: 10}}}
			follower := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.80, BestAskPx: 99.90, Levels: []model.Level{{Price: 99.90, Qty: 10}}}

			now := int64(1_000_000_000)
			if sig := e.Evaluate(now, leader, follower); sig != nil {
				return false
			}
			if sig := e.Evaluate(now+int64(persistMs-1)*1_000_000, leader, follower); sig != nil {
				return false
			}
			return e.Evaluate(now+int64(persistMs)*1_000_000, leader, follower) != nil
		},
		gen.IntRange(1, 500),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 13: Depth Filter Correctness**
// **Validates: Requirements 5.4**

func TestEngine_DepthFilter_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("Leader 深度不足时应过滤", prop.ForAll(
		func(minDepth float64) bool {
			if minDepth <= 0 {
				minDepth = 1
			}

			e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
				ThetaEntryBps:    10,
				PersistMs:        0,
				MinDepthUSD:      minDepth,
				VolFilterEnabled: false,
				CooldownMs:       0,
			})

			leader := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.01, Levels: []model.Level{{Price: 100, Qty: 0.0001}}}
			follower := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.80, BestAskPx: 99.90, Levels: []model.Level{{Price: 99.90, Qty: 10}}}

			// 由于 Leader 深度极小且 minDepth>0，必被过滤
			return e.Evaluate(1_000_000_000, leader, follower) == nil
		},
		gen.Float64Range(1, 1_000_000),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 14: Volatility Filter Correctness**
// **Validates: Requirements 5.5**

func TestEngine_VolatilityFilter_Property(t *testing.T) {
	e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
		ThetaEntryBps:    10,
		PersistMs:        1500,
		MinDepthUSD:      0,
		VolFilterEnabled: true,
		VolThreshold:     0, // 任意非零波动都应过滤
		CooldownMs:       0,
	})

	leader := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.01, Levels: []model.Level{{Price: 100, Qty: 10}}}
	follower := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.80, BestAskPx: 99.90, Levels: []model.Level{{Price: 99.90, Qty: 10}}}

	// 通过 1s 采样让 mid price 发生变化，形成非零 realized vol
	now := int64(1_000_000_000)
	leader.BestBidPx, leader.BestAskPx = 100.0, 100.01
	_ = e.Evaluate(now, leader, follower)

	now += int64(time.Second)
	leader.BestBidPx, leader.BestAskPx = 110.0, 110.01
	_ = e.Evaluate(now, leader, follower)

	now += int64(time.Second)
	leader.BestBidPx, leader.BestAskPx = 90.0, 90.01
	if sig := e.Evaluate(now, leader, follower); sig != nil {
		t.Fatalf("启用波动率过滤且阈值为 0 时不应产生信号")
	}
}

// **Feature: latency-arbitrage-validator, Property 15: Cooldown Correctness**
// **Validates: Requirements 5.6**

func TestEngine_Cooldown_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50
	properties := gopter.NewProperties(parameters)

	properties.Property("冷却期内不出信号，结束后允许", prop.ForAll(
		func(cooldownMs int) bool {
			if cooldownMs < 1 {
				cooldownMs = 1
			}
			e := NewEngine(model.ExchangeOKX, config.StrategyConfig{
				ThetaEntryBps:    10,
				PersistMs:        0,
				MinDepthUSD:      0,
				VolFilterEnabled: false,
				CooldownMs:       cooldownMs,
			})

			leader := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 100.00, BestAskPx: 100.01, Levels: []model.Level{{Price: 100, Qty: 10}}}
			follower := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.80, BestAskPx: 99.90, Levels: []model.Level{{Price: 99.90, Qty: 10}}}

			now := int64(1_000_000_000)
			e.NotifyStopLoss("BTCUSDT", now)

			if sig := e.Evaluate(now+1_000_000, leader, follower); sig != nil {
				return false
			}
			return e.Evaluate(now+int64(cooldownMs)*1_000_000, leader, follower) != nil
		},
		gen.IntRange(1, 10_000),
	))

	properties.TestingRun(t)
}
