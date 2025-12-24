// Package ev 实现 EV 相关的信号过滤逻辑。
package ev

import "latency-arbitrage-validator/internal/core/model"

// ApplyRejection 将 EV 结果应用到套利信号上
// 规则：当已有样本（Count>0）且 EV<0 时，标记信号为 RejectedByEV。
func ApplyRejection(sig *model.Signal, stats EVStats) {
	if sig == nil {
		return
	}
	if stats.Count > 0 && stats.EV < 0 {
		sig.RejectedByEV = true
		sig.FilterReason = "ev_negative"
	}
}
