// Package backoff 实现指数退避重连机制。
// 用于 WebSocket 断线重连时的延迟计算，避免频繁重连导致服务端拒绝。
// 基础间隔 1s，最大间隔 30s，抖动 ±20%
package backoff

import (
	"math/rand"
	"time"
)

// Backoff 指数退避计算器
// 每次调用 Next() 返回下一次重试的等待时间
// 等待时间按指数增长，直到达到最大值
type Backoff struct {
	// base 基础等待时间
	base time.Duration
	// max 最大等待时间
	max time.Duration
	// jitter 抖动比例（0-1），例如 0.2 表示 ±20%
	jitter float64
	// attempt 当前重试次数
	attempt int
}

// New 创建新的退避计算器
// 参数 base: 基础等待时间（建议 1s）
// 参数 max: 最大等待时间（建议 30s）
// 参数 jitter: 抖动比例（建议 0.2，即 ±20%）
func New(base, max time.Duration, jitter float64) *Backoff {
	return &Backoff{
		base:    base,
		max:     max,
		jitter:  jitter,
		attempt: 0,
	}
}

// NewDefault 创建默认配置的退避计算器
// 基础间隔 1s，最大间隔 30s，抖动 ±20%
func NewDefault() *Backoff {
	return New(time.Second, 30*time.Second, 0.2)
}

// Next 获取下次重试的等待时间
// 计算公式: base * 2^attempt，然后应用抖动
// 返回值不会超过 max
func (b *Backoff) Next() time.Duration {
	// 计算指数退避基础值: base * 2^attempt
	// 使用位移运算避免浮点数计算
	multiplier := int64(1) << b.attempt
	delay := b.base * time.Duration(multiplier)

	// 限制最大值
	if delay > b.max {
		delay = b.max
	}

	// 应用抖动: delay * (1 ± jitter)
	// 抖动范围: [delay * (1 - jitter), delay * (1 + jitter)]
	if b.jitter > 0 {
		// 生成 [-jitter, +jitter] 范围的随机数
		jitterFactor := 1.0 + (rand.Float64()*2-1)*b.jitter
		delay = time.Duration(float64(delay) * jitterFactor)
	}

	// 增加重试次数（用于下次计算）
	b.attempt++

	return delay
}

// Reset 重置退避计算器
// 在连接成功后调用，重置重试次数
func (b *Backoff) Reset() {
	b.attempt = 0
}

// Attempt 获取当前重试次数
func (b *Backoff) Attempt() int {
	return b.attempt
}
