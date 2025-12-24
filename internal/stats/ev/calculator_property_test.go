// Package ev EV 计算器属性测试
package ev

import (
	"math"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"latency-arbitrage-validator/internal/core/model"
)

// **Feature: latency-arbitrage-validator, Property 16: Rolling Statistics Correctness**
// **Validates: Requirements 7.1**

func TestCalculator_RollingStats_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 80
	properties := gopter.NewProperties(parameters)

	properties.Property("Stats 与手工聚合一致（window>=n）", prop.ForAll(
		func(grosses []float64, fees []float64) bool {
			if len(grosses) == 0 || len(fees) == 0 {
				return true
			}
			n := len(grosses)
			if len(fees) < n {
				n = len(fees)
			}
			if n == 0 {
				return true
			}

			c := NewCalculator(n + 10)

			var count, winCount, lossCount int64
			var sumWinR, sumLossL, sumFee float64

			for i := 0; i < n; i++ {
				g := clamp(grosses[i], -5000, 5000)
				f := clamp(fees[i], 0, 1000)
				net := g - f
				pos := &model.Position{Closed: true, GrossPnLBps: g, FeeBps: f, NetPnLBps: net}
				c.Add(pos)

				count++
				sumFee += f
				if net > 0 {
					winCount++
					sumWinR += g
				} else {
					lossCount++
					sumLossL += math.Abs(g)
				}
			}

			stats := c.Stats()
			if stats.Count != count || stats.WinCount != winCount || stats.LossCount != lossCount {
				return false
			}

			wantP := float64(winCount) / float64(count)
			wantF := sumFee / float64(count)
			var wantR, wantL float64
			if winCount > 0 {
				wantR = sumWinR / float64(winCount)
			}
			if lossCount > 0 {
				wantL = sumLossL / float64(lossCount)
			}

			if !approx(stats.WinRate, wantP, 1e-9) || !approx(stats.FeeBps, wantF, 1e-9) || !approx(stats.AvgProfit, wantR, 1e-9) || !approx(stats.AvgLoss, wantL, 1e-9) {
				return false
			}

			// EV 公式
			wantEV := wantP*(wantR-wantF) + (1-wantP)*(-wantL-wantF)
			if !approx(stats.EV, wantEV, 1e-9) {
				return false
			}

			return true
		},
		gen.SliceOfN(20, gen.Float64Range(-5000, 5000)),
		gen.SliceOfN(20, gen.Float64Range(0, 1000)),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 17: EV Formula Correctness**
// **Validates: Requirements 7.2, 7.3**

func TestCalculator_EVFormula_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 80
	properties := gopter.NewProperties(parameters)

	properties.Property("Stats.EV 与公式一致", prop.ForAll(
		func(grosses []float64, fees []float64) bool {
			n := len(grosses)
			if len(fees) < n {
				n = len(fees)
			}
			if n < 2 {
				return true
			}

			c := NewCalculator(n)
			for i := 0; i < n; i++ {
				g := clamp(grosses[i], -5000, 5000)
				f := clamp(fees[i], 0, 1000)
				net := g - f
				c.Add(&model.Position{Closed: true, GrossPnLBps: g, FeeBps: f, NetPnLBps: net})
			}

			s := c.Stats()
			p := s.WinRate
			R := s.AvgProfit
			L := s.AvgLoss
			f := s.FeeBps
			wantEV := p*(R-f) + (1-p)*(-L-f)

			if !approx(s.EV, wantEV, 1e-9) {
				return false
			}

			den := R + L
			var wantPReq float64
			if den > 0 {
				wantPReq = (L + f) / den
			} else {
				wantPReq = 1
			}
			return approx(s.PRequired, wantPReq, 1e-9)
		},
		gen.SliceOfN(30, gen.Float64Range(-5000, 5000)),
		gen.SliceOfN(30, gen.Float64Range(0, 1000)),
	))

	properties.TestingRun(t)
}

// **Feature: latency-arbitrage-validator, Property 18: EV Rejection Correctness**
// **Validates: Requirements 7.4**

func TestApplyRejection_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 80
	properties := gopter.NewProperties(parameters)

	properties.Property("EV<0 且 Count>0 时应拒绝", prop.ForAll(
		func(evValue float64, count int) bool {
			if count < 0 {
				count = -count
			}
			stats := EVStats{Count: int64(count), EV: evValue}
			sig := &model.Signal{}

			ApplyRejection(sig, stats)

			wantRejected := int64(count) > 0 && evValue < 0
			if sig.RejectedByEV != wantRejected {
				return false
			}
			if wantRejected && sig.FilterReason != "ev_negative" {
				return false
			}
			return true
		},
		gen.Float64Range(-1000, 1000),
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}

func clamp(v float64, lo float64, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func approx(a float64, b float64, eps float64) bool {
	return math.Abs(a-b) <= eps
}
