// Package signal 实现套利信号检测和过滤。
package signal

import (
	"fmt"
	"math"
	"time"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/util/timeutil"
)

type candidateState struct {
	active   bool
	startNs  int64
	signaled bool
}

type volState struct {
	lastSampleNs int64
	samples      []float64
	maxSamples   int
}

type symbolState struct {
	longCand  candidateState
	shortCand candidateState

	vol volState

	// cooldownUntilNs 止损冷却到期时间（纳秒）
	cooldownUntilNs int64
}

// Engine 信号引擎（单交易所 Leader 链路）
// 每个 Leader（OKX/Binance）应创建独立实例，避免状态混用。
type Engine struct {
	// leader 领先交易所: okx 或 binance
	leader string
	// cfg 策略配置
	cfg config.StrategyConfig

	// persistNs 持续时间过滤（纳秒）
	persistNs int64

	// states 按交易对维护状态
	states map[string]*symbolState
}

// NewEngine 创建信号引擎
// 参数 leader: okx 或 binance
// 参数 cfg: 策略配置
func NewEngine(leader string, cfg config.StrategyConfig) *Engine {
	e := &Engine{
		leader:    leader,
		cfg:       cfg,
		persistNs: int64(cfg.PersistMs) * 1_000_000,
		states:    make(map[string]*symbolState),
	}
	return e
}

// NotifyStopLoss 通知引擎发生止损，用于触发冷却窗口
// 参数 symbolCanon: 统一交易对
// 参数 nowNs: 当前时间（纳秒）
func (e *Engine) NotifyStopLoss(symbolCanon string, nowNs int64) {
	st := e.getState(symbolCanon)
	st.cooldownUntilNs = nowNs + int64(e.cfg.CooldownMs)*1_000_000
}

// Evaluate 评估当前 Leader/Follower 订单簿是否触发信号
// 返回值：若触发并通过过滤器返回 Signal，否则返回 nil。
func (e *Engine) Evaluate(nowNs int64, leaderBook, followerBook *model.BookEvent) *model.Signal {
	if leaderBook == nil || followerBook == nil {
		return nil
	}
	if leaderBook.Exchange != e.leader {
		return nil
	}
	if followerBook.Exchange != model.ExchangeBittap {
		return nil
	}
	if leaderBook.SymbolCanon == "" || followerBook.SymbolCanon == "" || leaderBook.SymbolCanon != followerBook.SymbolCanon {
		return nil
	}
	if !leaderBook.IsValid() || !followerBook.IsValid() {
		return nil
	}

	st := e.getState(leaderBook.SymbolCanon)

	// 止损冷却过滤：在冷却期内不产生新信号
	if st.cooldownUntilNs > 0 && nowNs < st.cooldownUntilNs {
		return nil
	}

	// 深度过滤：Leader 前 5 档名义价值必须达到阈值
	if e.cfg.MinDepthUSD > 0 && leaderBook.Top5DepthUSD() < e.cfg.MinDepthUSD {
		e.resetCandidates(st)
		return nil
	}

	// 波动率过滤：1min realized vol 超阈值跳过（可关闭）
	if e.cfg.VolFilterEnabled {
		e.updateVol(st, nowNs, leaderBook.MidPrice())
		if e.realizedVol(st) > e.cfg.VolThreshold {
			return nil
		}
	}

	// 计算多头信号：Leader_bid - Follower_ask > θ_entry
	longBps, longOK := calcLongSpreadBps(leaderBook, followerBook)
	if longOK && longBps > e.cfg.ThetaEntryBps {
		if sig := e.tryFire(nowNs, leaderBook, followerBook, model.SideLong, longBps, &st.longCand); sig != nil {
			return sig
		}
	} else {
		st.longCand = candidateState{}
	}

	// 计算空头信号：Follower_bid - Leader_ask > θ_entry
	shortBps, shortOK := calcShortSpreadBps(leaderBook, followerBook)
	if shortOK && shortBps > e.cfg.ThetaEntryBps {
		if sig := e.tryFire(nowNs, leaderBook, followerBook, model.SideShort, shortBps, &st.shortCand); sig != nil {
			return sig
		}
	} else {
		st.shortCand = candidateState{}
	}

	return nil
}

func (e *Engine) getState(symbolCanon string) *symbolState {
	st, ok := e.states[symbolCanon]
	if ok {
		return st
	}
	st = &symbolState{
		vol: volState{
			maxSamples: 60, // 1 分钟：按 1s 采样
		},
	}
	e.states[symbolCanon] = st
	return st
}

func (e *Engine) resetCandidates(st *symbolState) {
	st.longCand = candidateState{}
	st.shortCand = candidateState{}
}

func (e *Engine) tryFire(nowNs int64, leaderBook, followerBook *model.BookEvent, side model.Side, spreadBps float64, cand *candidateState) *model.Signal {
	if !cand.active {
		cand.active = true
		cand.startNs = nowNs
		cand.signaled = false

		// persist=0 表示不需要持续性过滤，首次满足条件即触发。
		if e.persistNs == 0 {
			cand.signaled = true
			id := fmt.Sprintf("%s-%s-%s-%d", e.leader, leaderBook.SymbolCanon, side, nowNs)
			return &model.Signal{
				ID:           id,
				Leader:       e.leader,
				SymbolCanon:  leaderBook.SymbolCanon,
				Side:         side,
				SpreadBps:    spreadBps,
				LeaderBook:   leaderBook.Clone(),
				FollowerBook: followerBook.Clone(),
				DetectedAt:   timeutil.NanoToTime(nowNs),
				DetectedAtNs: nowNs,
			}
		}

		return nil
	}
	if cand.signaled {
		return nil
	}
	if nowNs-cand.startNs < e.persistNs {
		return nil
	}

	cand.signaled = true

	id := fmt.Sprintf("%s-%s-%s-%d", e.leader, leaderBook.SymbolCanon, side, nowNs)
	return &model.Signal{
		ID:           id,
		Leader:       e.leader,
		SymbolCanon:  leaderBook.SymbolCanon,
		Side:         side,
		SpreadBps:    spreadBps,
		LeaderBook:   leaderBook.Clone(),
		FollowerBook: followerBook.Clone(),
		DetectedAt:   timeutil.NanoToTime(nowNs),
		DetectedAtNs: nowNs,
	}
}

func calcLongSpreadBps(leaderBook, followerBook *model.BookEvent) (float64, bool) {
	if leaderBook.BestBidPx <= 0 || followerBook.BestAskPx <= 0 {
		return 0, false
	}
	return (leaderBook.BestBidPx - followerBook.BestAskPx) / followerBook.BestAskPx * 10000, true
}

func calcShortSpreadBps(leaderBook, followerBook *model.BookEvent) (float64, bool) {
	if followerBook.BestBidPx <= 0 || leaderBook.BestAskPx <= 0 {
		return 0, false
	}
	return (followerBook.BestBidPx - leaderBook.BestAskPx) / leaderBook.BestAskPx * 10000, true
}

// updateVol 更新 1 分钟 realized vol 的采样序列（1s 采样）
func (e *Engine) updateVol(st *symbolState, nowNs int64, midPx float64) {
	if midPx <= 0 {
		return
	}
	if st.vol.lastSampleNs > 0 && nowNs-st.vol.lastSampleNs < int64(time.Second) {
		return
	}
	st.vol.lastSampleNs = nowNs

	st.vol.samples = append(st.vol.samples, midPx)
	if len(st.vol.samples) > st.vol.maxSamples {
		st.vol.samples = st.vol.samples[len(st.vol.samples)-st.vol.maxSamples:]
	}
}

// realizedVol 计算 1 分钟 realized volatility（log return 的标准差）
// 返回值越大表示波动越大；本实现为验证阶段的轻量版本。
func (e *Engine) realizedVol(st *symbolState) float64 {
	n := len(st.vol.samples)
	if n < 2 {
		return 0
	}

	// 计算 log returns
	returns := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		p0 := st.vol.samples[i-1]
		p1 := st.vol.samples[i]
		if p0 <= 0 || p1 <= 0 {
			continue
		}
		returns = append(returns, math.Log(p1/p0))
	}
	if len(returns) < 2 {
		return 0
	}

	var sum float64
	for _, r := range returns {
		sum += r
	}
	mean := sum / float64(len(returns))

	var ss float64
	for _, r := range returns {
		d := r - mean
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(returns)-1))
}
