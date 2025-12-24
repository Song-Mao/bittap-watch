// Package latency 时延追踪器测试
package latency

import (
	"math"
	"sort"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/util/timeutil"
)

// **Feature: latency-arbitrage-validator, Property 8: Lag Calculation Correctness**
// **Validates: Requirements 4.1, 4.2**

func TestTracker_LagCalculation(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("arrival-based lag 计算正确", prop.ForAll(
		func(leaderArrivedNs, followerArrivedNs int64) bool {
			if leaderArrivedNs < 0 {
				leaderArrivedNs = -leaderArrivedNs
			}
			if followerArrivedNs < leaderArrivedNs {
				followerArrivedNs = leaderArrivedNs + (leaderArrivedNs%1_000_000 + 1)
			}

			tr := NewTracker(100)

			leader := &model.BookEvent{
				Exchange:        model.ExchangeOKX,
				SymbolCanon:     "BTCUSDT",
				ArrivedAtUnixNs: leaderArrivedNs,
				ExchTsUnixMs:    1700000000000,
			}
			follower := &model.BookEvent{
				Exchange:        model.ExchangeBittap,
				SymbolCanon:     "BTCUSDT",
				ArrivedAtUnixNs: followerArrivedNs,
			}

			tr.Add(leader, follower)
			stats := tr.Stats(model.ExchangeOKX)

			wantMs := float64(followerArrivedNs-leaderArrivedNs) / 1_000_000.0
			return approxEqual(stats.ArrivedP50Ms, wantMs, 1e-9) && approxEqual(stats.ArrivedP90Ms, wantMs, 1e-9) && approxEqual(stats.ArrivedP99Ms, wantMs, 1e-9)
		},
		gen.Int64(),
		gen.Int64(),
	))

	properties.Property("event-based lag 计算正确（Leader 有 ExchTs 时）", prop.ForAll(
		func(leaderExchTsMs, followerArrivedNs int64) bool {
			if leaderExchTsMs <= 0 {
				leaderExchTsMs = 1700000000000
			}
			if followerArrivedNs <= 0 {
				followerArrivedNs = timeutil.MsToNano(leaderExchTsMs) + 10_000_000
			}

			tr := NewTracker(100)

			leader := &model.BookEvent{
				Exchange:     model.ExchangeBinance,
				SymbolCanon:  "BTCUSDT",
				ExchTsUnixMs: leaderExchTsMs,
			}
			follower := &model.BookEvent{
				Exchange:        model.ExchangeBittap,
				SymbolCanon:     "BTCUSDT",
				ArrivedAtUnixNs: followerArrivedNs,
			}

			tr.Add(leader, follower)
			stats := tr.Stats(model.ExchangeBinance)

			wantMs := float64(followerArrivedNs-timeutil.MsToNano(leaderExchTsMs)) / 1_000_000.0
			return approxEqual(stats.EventP50Ms, wantMs, 1e-9) && approxEqual(stats.EventP90Ms, wantMs, 1e-9) && approxEqual(stats.EventP99Ms, wantMs, 1e-9)
		},
		gen.Int64Range(1700000000000, 1800000000000),
		gen.Int64(),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 9: Percentile Calculation Correctness**
// **Validates: Requirements 4.3**

func TestTracker_Percentiles(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	properties.Property("P50/P90/P99 与排序分位数一致", prop.ForAll(
		func(lagsMs []int64) bool {
			if len(lagsMs) < 3 {
				return true
			}

			tr := NewTracker(1000)
			for i, ms := range lagsMs {
				if ms < 0 {
					ms = -ms
				}
				leader := &model.BookEvent{
					Exchange:        model.ExchangeOKX,
					SymbolCanon:     "BTCUSDT",
					ArrivedAtUnixNs: int64(i) * 1_000_000_000,
				}
				follower := &model.BookEvent{
					Exchange:        model.ExchangeBittap,
					SymbolCanon:     "BTCUSDT",
					ArrivedAtUnixNs: leader.ArrivedAtUnixNs + ms*1_000_000,
				}
				tr.Add(leader, follower)
			}

			stats := tr.Stats(model.ExchangeOKX)

			sorted := make([]int64, len(lagsMs))
			copy(sorted, lagsMs)
			for i := range sorted {
				if sorted[i] < 0 {
					sorted[i] = -sorted[i]
				}
			}
			sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

			want50 := float64(sorted[idxQuantile(sorted, 0.50)])
			want90 := float64(sorted[idxQuantile(sorted, 0.90)])
			want99 := float64(sorted[idxQuantile(sorted, 0.99)])

			return approxEqual(stats.ArrivedP50Ms, want50, 1e-9) &&
				approxEqual(stats.ArrivedP90Ms, want90, 1e-9) &&
				approxEqual(stats.ArrivedP99Ms, want99, 1e-9)
		},
		gen.SliceOfN(20, gen.Int64Range(0, 5000)),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 10: Leader Independence**
// **Validates: Requirements 4.4**

func TestTracker_LeaderIndependence(t *testing.T) {
	tr := NewTracker(100)

	// OKX: 10ms
	tr.Add(
		&model.BookEvent{Exchange: model.ExchangeOKX, SymbolCanon: "BTCUSDT", ArrivedAtUnixNs: 0, ExchTsUnixMs: 1700000000000},
		&model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", ArrivedAtUnixNs: 10 * 1_000_000},
	)
	// Binance: 100ms
	tr.Add(
		&model.BookEvent{Exchange: model.ExchangeBinance, SymbolCanon: "BTCUSDT", ArrivedAtUnixNs: 0, ExchTsUnixMs: 1700000000000},
		&model.BookEvent{Exchange: model.ExchangeBittap, SymbolCanon: "BTCUSDT", ArrivedAtUnixNs: 100 * 1_000_000},
	)

	okxStats := tr.Stats(model.ExchangeOKX)
	binStats := tr.Stats(model.ExchangeBinance)

	if math.Abs(okxStats.ArrivedP50Ms-10) > 1e-9 {
		t.Fatalf("okx ArrivedP50Ms=%f, want 10", okxStats.ArrivedP50Ms)
	}
	if math.Abs(binStats.ArrivedP50Ms-100) > 1e-9 {
		t.Fatalf("binance ArrivedP50Ms=%f, want 100", binStats.ArrivedP50Ms)
	}
}

func idxQuantile(sorted []int64, q float64) int {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return 0
	}
	if q >= 1 {
		return len(sorted) - 1
	}
	idx := int(float64(len(sorted)-1) * q)
	if idx < 0 {
		return 0
	}
	if idx >= len(sorted) {
		return len(sorted) - 1
	}
	return idx
}

func approxEqual(a, b float64, eps float64) bool {
	return math.Abs(a-b) <= eps
}
