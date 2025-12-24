// Package timeutil 提供时间相关的工具函数。
// 主要用于获取高精度时间戳，用于延迟测量和事件记录。
package timeutil

import (
	"time"
)

var (
	// baseTime 基准时间点（包含单调时钟读数）
	baseTime = time.Now()
	// baseUnixNs 基准时间点对应的 Unix 纳秒时间戳
	baseUnixNs = baseTime.UnixNano()
)

// NowNano 获取当前时间的纳秒时间戳
// 使用“单调时钟 + 启动时 Unix 时间”组合实现：
// NowNano = baseUnixNs + time.Since(baseTime).Nanoseconds()
// 这样在系统时间跳变（NTP/手动调整）时也能保持时间差的单调性，避免污染 lead-lag 统计。
// 返回: 当前时间的 Unix 纳秒时间戳
func NowNano() int64 {
	return baseUnixNs + time.Since(baseTime).Nanoseconds()
}

// NowMs 获取当前时间的毫秒时间戳
// 用于与交易所时间戳对比（交易所通常使用毫秒）
// 返回: 当前时间的 Unix 毫秒时间戳
func NowMs() int64 {
	return NowNano() / 1_000_000
}

// NowMicro 获取当前时间的微秒时间戳
// 用于需要微秒精度的场景
// 返回: 当前时间的 Unix 微秒时间戳
func NowMicro() int64 {
	return NowNano() / 1_000
}

// NanoToMs 将纳秒时间戳转换为毫秒
// 参数 ns: 纳秒时间戳
// 返回: 毫秒时间戳
func NanoToMs(ns int64) int64 {
	return ns / 1_000_000
}

// MsToNano 将毫秒时间戳转换为纳秒
// 参数 ms: 毫秒时间戳
// 返回: 纳秒时间戳
func MsToNano(ms int64) int64 {
	return ms * 1_000_000
}

// NanoToTime 将纳秒时间戳转换为 time.Time
// 参数 ns: 纳秒时间戳
// 返回: time.Time 对象
func NanoToTime(ns int64) time.Time {
	return time.Unix(0, ns)
}

// MsToTime 将毫秒时间戳转换为 time.Time
// 参数 ms: 毫秒时间戳
// 返回: time.Time 对象
func MsToTime(ms int64) time.Time {
	return time.UnixMilli(ms)
}

// DurationMs 计算两个纳秒时间戳之间的毫秒差
// 参数 startNs: 开始时间（纳秒）
// 参数 endNs: 结束时间（纳秒）
// 返回: 时间差（毫秒，浮点数以保留精度）
func DurationMs(startNs, endNs int64) float64 {
	return float64(endNs-startNs) / 1_000_000.0
}

// SinceNano 计算从指定纳秒时间戳到现在的时间差
// 参数 startNs: 开始时间（纳秒）
// 返回: 时间差（time.Duration）
func SinceNano(startNs int64) time.Duration {
	return time.Duration(NowNano() - startNs)
}

// SinceMs 计算从指定毫秒时间戳到现在的时间差
// 参数 startMs: 开始时间（毫秒）
// 返回: 时间差（毫秒）
func SinceMs(startMs int64) int64 {
	return NowMs() - startMs
}
