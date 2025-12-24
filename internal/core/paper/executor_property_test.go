// Package paper 影子成交执行器属性测试
package paper

import (
	"math"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
)

// **Feature: latency-arbitrage-validator, Property 16: Entry Price Correctness**
// **Validates: Requirements 6.1**

func TestExecutor_EntryPrice_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("long 入场价=ask*(1+slip), short 入场价=bid*(1-slip)", prop.ForAll(
		func(ask, bid float64, slipBps float64) bool {
			if ask <= 0 {
				ask = 100
			}
			if bid <= 0 || bid >= ask {
				bid = ask - 0.01
			}
			if slipBps < 0 {
				slipBps = -slipBps
			}
			if slipBps > 100 {
				slipBps = 100
			}

			cfg := config.PaperConfig{TPRatio: 0.0, SLRatio: 0.0, MaxHoldMs: 60000, SlippageBps: slipBps}
			exec := NewExecutor(model.ExchangeOKX, cfg, config.FeeDetail{})

			leader := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: ask + 1, BestAskPx: ask + 1.01}
			follower := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: bid, BestAskPx: ask}

			slip := slipBps / 10000

			longSig := &model.Signal{Leader: model.ExchangeOKX, SymbolCanon: "BTCUSDT", Side: model.SideLong, SpreadBps: 100, DetectedAtNs: 1, LeaderBook: leader, FollowerBook: follower}
			longPos, opened, err := exec.TryOpen(longSig)
			if err != nil || !opened || longPos == nil {
				return false
			}
			if !approx(longPos.EntryPx, ask*(1+slip), 1e-9) {
				return false
			}

			// short 用另一个 symbol，避免被“已有持仓”拦截
			exec2 := NewExecutor(model.ExchangeOKX, cfg, config.FeeDetail{})
			shortSig := &model.Signal{Leader: model.ExchangeOKX, SymbolCanon: "ETHUSDT", Side: model.SideShort, SpreadBps: 100, DetectedAtNs: 1, LeaderBook: leader, FollowerBook: &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "ETHUSDT", BestBidPx: bid, BestAskPx: ask}}
			shortPos, opened, err := exec2.TryOpen(shortSig)
			if err != nil || !opened || shortPos == nil {
				return false
			}
			return approx(shortPos.EntryPx, bid*(1-slip), 1e-9)
		},
		gen.Float64Range(1, 200000),
		gen.Float64Range(1, 200000),
		gen.Float64Range(0, 100),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 17: Fee Calculation Correctness**
// **Validates: Requirements 6.2**

func TestExecutor_FeeBps_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("FeeBps=2*effective_taker_fee*10000", prop.ForAll(
		func(taker, rebate float64) bool {
			if taker < 0 {
				taker = -taker
			}
			if rebate < 0 {
				rebate = -rebate
			}
			if taker > 1 {
				taker = 1
			}
			if rebate > 1 {
				rebate = 1
			}

			fee := config.FeeDetail{TakerRate: taker, RebateRate: rebate}
			exec := NewExecutor(model.ExchangeOKX, config.PaperConfig{MaxHoldMs: 60000}, fee)

			sig := &model.Signal{
				Leader:       model.ExchangeOKX,
				SymbolCanon:  "BTCUSDT",
				Side:         model.SideLong,
				SpreadBps:    100,
				DetectedAtNs: 1,
				LeaderBook:   &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 101, BestAskPx: 101.01},
				FollowerBook: &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.99, BestAskPx: 100},
			}

			pos, opened, err := exec.TryOpen(sig)
			if err != nil || !opened || pos == nil {
				return false
			}

			want := 2 * (taker * (1 - rebate)) * 10000
			return approx(pos.FeeBps, want, 1e-9)
		},
		gen.Float64Range(0, 1),
		gen.Float64Range(0, 1),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 18: Exit Condition Correctness**
// **Validates: Requirements 6.3, 6.4, 6.5**

func TestExecutor_ExitConditions_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 80
	properties := gopter.NewProperties(parameters)

	properties.Property("TP/SL 阈值触发正确（long）", prop.ForAll(
		func(entrySpreadBps float64, tpRatio float64, slRatio float64) bool {
			if entrySpreadBps < 10 {
				entrySpreadBps = 10
			}
			if tpRatio <= 0 {
				tpRatio = 0.1
			}
			if tpRatio > 0.9 {
				tpRatio = 0.9
			}
			if slRatio <= 0 {
				slRatio = 0.1
			}
			if slRatio > 1.0 {
				slRatio = 1.0
			}

			exec := NewExecutor(model.ExchangeOKX, config.PaperConfig{
				TPRatio:   tpRatio,
				SLRatio:   slRatio,
				MaxHoldMs: 60000,
			}, config.FeeDetail{})

			leader := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: 101, BestAskPx: 101.01}
			follower := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: 99.99, BestAskPx: 100}

			sig := &model.Signal{
				Leader:       model.ExchangeOKX,
				SymbolCanon:  "BTCUSDT",
				Side:         model.SideLong,
				SpreadBps:    entrySpreadBps,
				DetectedAtNs: 1_000_000_000,
				LeaderBook:   leader,
				FollowerBook: follower,
			}
			_, opened, err := exec.TryOpen(sig)
			if err != nil || !opened {
				return false
			}

			// 构造一个满足 TP 的 current spread
			curTP := (1.0 - tpRatio) * entrySpreadBps * 0.9
			if curTP < 0 {
				curTP = 0
			}
			ask := 100.0
			leaderTPBid := ask * (1 + curTP/10000)
			leaderNow := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: leaderTPBid, BestAskPx: leaderTPBid + 0.01}
			followerNow := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: ask - 0.01, BestAskPx: ask}
			closed := exec.Evaluate(2_000_000_000, leaderNow, followerNow)
			if closed == nil || closed.ExitReason != model.ExitTP {
				return false
			}

			// 重新开一笔测试 SL
			exec2 := NewExecutor(model.ExchangeOKX, config.PaperConfig{
				TPRatio:   tpRatio,
				SLRatio:   slRatio,
				MaxHoldMs: 60000,
			}, config.FeeDetail{})
			_, opened, err = exec2.TryOpen(sig)
			if err != nil || !opened {
				return false
			}

			curSL := (1.0 + slRatio) * entrySpreadBps * 1.1
			leaderSLBid := ask * (1 + curSL/10000)
			leaderNow2 := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: leaderSLBid, BestAskPx: leaderSLBid + 0.01}
			closed2 := exec2.Evaluate(2_000_000_000, leaderNow2, followerNow)
			return closed2 != nil && closed2.ExitReason == model.ExitSL
		},
		gen.Float64Range(10, 2000),
		gen.Float64Range(0.1, 0.9),
		gen.Float64Range(0.1, 1.0),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 19: PnL Calculation Correctness**
// **Validates: Requirements 6.6**

func TestExecutor_PnL_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("gross/net PnL bps 公式正确（timeout 强制平仓）", prop.ForAll(
		func(entryPx float64, moveBps float64) bool {
			if entryPx <= 0 {
				entryPx = 100
			}
			if moveBps < -5000 {
				moveBps = -5000
			}
			if moveBps > 5000 {
				moveBps = 5000
			}

			exec := NewExecutor(model.ExchangeOKX, config.PaperConfig{
				MaxHoldMs:   1,
				SlippageBps: 0,
			}, config.FeeDetail{})

			sig := &model.Signal{
				Leader:       model.ExchangeOKX,
				SymbolCanon:  "BTCUSDT",
				Side:         model.SideLong,
				SpreadBps:    100,
				DetectedAtNs: 1_000_000_000,
				LeaderBook:   &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: entryPx + 1, BestAskPx: entryPx + 1.01},
				FollowerBook: &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: entryPx - 0.01, BestAskPx: entryPx},
			}
			pos, opened, err := exec.TryOpen(sig)
			if err != nil || !opened || pos == nil {
				return false
			}

			exitPx := pos.EntryPx * (1 + moveBps/10000)
			leaderNow := &model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", BestBidPx: entryPx + 1, BestAskPx: entryPx + 1.01}
			followerNow := &model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", BestBidPx: exitPx, BestAskPx: exitPx + 0.01}
			closed := exec.Evaluate(pos.EntryTimeNs+2_000_000, leaderNow, followerNow)
			if closed == nil || !closed.Closed {
				return false
			}

			wantGross := (exitPx - pos.EntryPx) / pos.EntryPx * 10000
			wantNet := wantGross - pos.FeeBps

			return approx(closed.GrossPnLBps, wantGross, 1e-6) && approx(closed.NetPnLBps, wantNet, 1e-6)
		},
		gen.Float64Range(1, 200000),
		gen.Float64Range(-2000, 2000),
	))

	properties.TestingRun(t)
}

func approx(a, b float64, eps float64) bool {
	return math.Abs(a-b) <= eps
}
