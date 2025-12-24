// Package latency 实现 lead-lag 时延测量和统计。
// 为 OKX→Bittap 和 Binance→Bittap 维护独立的追踪器。
package latency

import (
	"sort"
	"sync"

	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/util/timeutil"
)

// LatencyStats 时延统计快照（滚动窗口）
// 单位：毫秒。
type LatencyStats struct {
	// Leader 领先交易所: okx 或 binance
	Leader string
	// Count 样本总数（累计）
	Count int64

	// ArrivedP50Ms 基于到达时间的 P50 时延（毫秒）
	ArrivedP50Ms float64
	// ArrivedP90Ms 基于到达时间的 P90 时延（毫秒）
	ArrivedP90Ms float64
	// ArrivedP99Ms 基于到达时间的 P99 时延（毫秒）
	ArrivedP99Ms float64

	// EventP50Ms 基于交易所事件时间的 P50 时延（毫秒）
	EventP50Ms float64
	// EventP90Ms 基于交易所事件时间的 P90 时延（毫秒）
	EventP90Ms float64
	// EventP99Ms 基于交易所事件时间的 P99 时延（毫秒）
	EventP99Ms float64
}

type rollingWindow struct {
	size  int
	buf   []int64
	pos   int
	count int64
	full  bool

	mu sync.Mutex
}

func newRollingWindow(size int) *rollingWindow {
	return &rollingWindow{size: size, buf: make([]int64, 0, size)}
}

func (w *rollingWindow) add(v int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.count++
	if w.size <= 0 {
		return
	}

	if !w.full {
		w.buf = append(w.buf, v)
		if len(w.buf) == w.size {
			w.full = true
			w.pos = 0
		}
		return
	}

	w.buf[w.pos] = v
	w.pos++
	if w.pos >= w.size {
		w.pos = 0
	}
}

func (w *rollingWindow) snapshotQuantiles(qs ...float64) (count int64, values []int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	count = w.count
	if len(w.buf) == 0 {
		return count, make([]int64, len(qs))
	}

	tmp := make([]int64, len(w.buf))
	copy(tmp, w.buf)
	sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })

	values = make([]int64, len(qs))
	n := len(tmp)
	for i, q := range qs {
		if q <= 0 {
			values[i] = tmp[0]
			continue
		}
		if q >= 1 {
			values[i] = tmp[n-1]
			continue
		}
		idx := int(float64(n-1) * q)
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		values[i] = tmp[idx]
	}
	return count, values
}

type linkTracker struct {
	arrived *rollingWindow
	event   *rollingWindow
}

// Tracker 时延追踪器
// 为 OKX→Bittap 与 Binance→Bittap 维护独立的滚动窗口统计。
type Tracker struct {
	// okx OKX↙Bittap 链路统计
	okx linkTracker
	// binance Binance↙Bittap 链路统计
	binance linkTracker
}

// NewTracker 创建时延追踪器
// 参数 windowSize: 滚动窗口大小（建议 10000），用于 P50/P90/P99。
func NewTracker(windowSize int) *Tracker {
	return &Tracker{
		okx: linkTracker{
			arrived: newRollingWindow(windowSize),
			event:   newRollingWindow(windowSize),
		},
		binance: linkTracker{
			arrived: newRollingWindow(windowSize),
			event:   newRollingWindow(windowSize),
		},
	}
}

// Add 基于一对 Leader/Follower 的 BookEvent 更新统计
// 时延定义：
// - arrived_lag_ns = follower.ArrivedAtUnixNs - leader.ArrivedAtUnixNs
// - event_lag_ns = follower.ArrivedAtUnixNs - leader.ExchTsUnixMs（转换为 ns；若 ExchTsUnixMs<=0 则不记录）
func (t *Tracker) Add(leaderEv, followerEv *model.BookEvent) {
	if leaderEv == nil || followerEv == nil {
		return
	}
	if followerEv.Exchange != model.ExchangeBittap {
		return
	}
	if leaderEv.SymbolCanon == "" || followerEv.SymbolCanon == "" || leaderEv.SymbolCanon != followerEv.SymbolCanon {
		return
	}

	lagArrivedNs := followerEv.ArrivedAtUnixNs - leaderEv.ArrivedAtUnixNs
	var lagEventNs int64
	if leaderEv.ExchTsUnixMs > 0 {
		lagEventNs = followerEv.ArrivedAtUnixNs - timeutil.MsToNano(leaderEv.ExchTsUnixMs)
	} else {
		lagEventNs = 0
	}

	switch leaderEv.Exchange {
	case model.ExchangeOKX:
		t.okx.arrived.add(lagArrivedNs)
		if lagEventNs != 0 {
			t.okx.event.add(lagEventNs)
		}
	case model.ExchangeBinance:
		t.binance.arrived.add(lagArrivedNs)
		if lagEventNs != 0 {
			t.binance.event.add(lagEventNs)
		}
	}
}

// Stats 获取指定 Leader 的统计快照
// 参数 leader: okx 或 binance
func (t *Tracker) Stats(leader string) LatencyStats {
	var lt linkTracker
	switch leader {
	case model.ExchangeOKX:
		lt = t.okx
	case model.ExchangeBinance:
		lt = t.binance
	default:
		return LatencyStats{Leader: leader}
	}

	arrivedCount, arrivedQs := lt.arrived.snapshotQuantiles(0.50, 0.90, 0.99)
	eventCount, eventQs := lt.event.snapshotQuantiles(0.50, 0.90, 0.99)
	_ = eventCount

	return LatencyStats{
		Leader:       leader,
		Count:        arrivedCount,
		ArrivedP50Ms: float64(arrivedQs[0]) / 1_000_000.0,
		ArrivedP90Ms: float64(arrivedQs[1]) / 1_000_000.0,
		ArrivedP99Ms: float64(arrivedQs[2]) / 1_000_000.0,
		EventP50Ms:   float64(eventQs[0]) / 1_000_000.0,
		EventP90Ms:   float64(eventQs[1]) / 1_000_000.0,
		EventP99Ms:   float64(eventQs[2]) / 1_000_000.0,
	}
}
