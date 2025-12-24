// Package backoff 退避算法测试
package backoff

import (
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: latency-arbitrage-validator, Property 3: Exponential Backoff Bounds**
// **Validates: Requirements 1.3**

// TestBackoff_ExponentialGrowth 测试退避时间指数增长
func TestBackoff_ExponentialGrowth(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: 退避时间应该指数增长（在达到最大值之前）
	properties.Property("退避时间指数增长", prop.ForAll(
		func(baseMs int, maxMs int) bool {
			if baseMs <= 0 || maxMs <= baseMs {
				return true // 跳过无效输入
			}

			base := time.Duration(baseMs) * time.Millisecond
			max := time.Duration(maxMs) * time.Millisecond
			b := New(base, max, 0) // 无抖动，便于验证

			prev := time.Duration(0)
			for i := 0; i < 10; i++ {
				delay := b.Next()

				// 验证: 每次延迟应该 >= 前一次（指数增长）
				// 或者已经达到最大值
				if delay < prev && delay != max {
					return false
				}

				// 验证: 延迟不应超过最大值
				if delay > max {
					return false
				}

				prev = delay
			}
			return true
		},
		gen.IntRange(100, 2000),   // base: 100ms - 2s
		gen.IntRange(5000, 60000), // max: 5s - 60s
	))

	properties.TestingRun(t)
}

// TestBackoff_JitterBounds 测试抖动范围
func TestBackoff_JitterBounds(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: 抖动后的延迟应在 ±jitter 范围内
	properties.Property("抖动在指定范围内", prop.ForAll(
		func(jitterPercent int) bool {
			jitter := float64(jitterPercent) / 100.0 // 转换为 0-1 范围
			base := time.Second
			max := 30 * time.Second
			b := New(base, max, jitter)

			// 多次测试以验证抖动范围
			for i := 0; i < 50; i++ {
				b.Reset()
				delay := b.Next()

				// 计算期望的基础值（第一次调用，attempt=0，所以是 base）
				expectedBase := float64(base)
				minExpected := expectedBase * (1 - jitter)
				maxExpected := expectedBase * (1 + jitter)

				delayFloat := float64(delay)
				if delayFloat < minExpected || delayFloat > maxExpected {
					return false
				}
			}
			return true
		},
		gen.IntRange(0, 50), // jitter: 0% - 50%
	))

	properties.TestingRun(t)
}

// TestBackoff_MaxBound 测试最大值边界
func TestBackoff_MaxBound(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: 延迟永远不应超过最大值（考虑抖动）
	properties.Property("延迟不超过最大值上限", prop.ForAll(
		func(baseMs int, maxMs int, jitterPercent int) bool {
			if baseMs <= 0 || maxMs <= 0 {
				return true
			}

			base := time.Duration(baseMs) * time.Millisecond
			max := time.Duration(maxMs) * time.Millisecond
			jitter := float64(jitterPercent) / 100.0
			b := New(base, max, jitter)

			// 最大可能的延迟（考虑抖动）
			maxPossible := float64(max) * (1 + jitter)

			for i := 0; i < 20; i++ {
				delay := b.Next()
				if float64(delay) > maxPossible {
					return false
				}
			}
			return true
		},
		gen.IntRange(100, 2000),   // base
		gen.IntRange(1000, 60000), // max
		gen.IntRange(0, 30),       // jitter %
	))

	properties.TestingRun(t)
}

// TestBackoff_Reset 测试重置功能
func TestBackoff_Reset(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: 重置后应该从基础值重新开始
	properties.Property("重置后从基础值开始", prop.ForAll(
		func(attempts int) bool {
			if attempts <= 0 {
				return true
			}

			b := New(time.Second, 30*time.Second, 0) // 无抖动

			// 进行多次重试
			for i := 0; i < attempts; i++ {
				b.Next()
			}

			// 重置
			b.Reset()

			// 验证重试次数归零
			if b.Attempt() != 0 {
				return false
			}

			// 验证下次延迟回到基础值
			delay := b.Next()
			return delay == time.Second
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// TestBackoff_DefaultConfig 测试默认配置
func TestBackoff_DefaultConfig(t *testing.T) {
	b := NewDefault()

	// 验证默认配置: base=1s, max=30s, jitter=0.2
	if b.base != time.Second {
		t.Errorf("默认 base = %v, want 1s", b.base)
	}
	if b.max != 30*time.Second {
		t.Errorf("默认 max = %v, want 30s", b.max)
	}
	if b.jitter != 0.2 {
		t.Errorf("默认 jitter = %v, want 0.2", b.jitter)
	}
}

// TestBackoff_SpecificValues 测试特定值（单元测试）
func TestBackoff_SpecificValues(t *testing.T) {
	// 无抖动的情况下验证指数增长
	b := New(time.Second, 30*time.Second, 0)

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, time.Second},      // 2^0 = 1
		{1, 2 * time.Second},  // 2^1 = 2
		{2, 4 * time.Second},  // 2^2 = 4
		{3, 8 * time.Second},  // 2^3 = 8
		{4, 16 * time.Second}, // 2^4 = 16
		{5, 30 * time.Second}, // 2^5 = 32, 但限制为 30
		{6, 30 * time.Second}, // 继续保持最大值
	}

	for _, tt := range tests {
		b.Reset()
		// 跳过到指定的 attempt
		for i := 0; i < tt.attempt; i++ {
			b.Next()
		}
		got := b.Next()
		if got != tt.expected {
			t.Errorf("attempt %d: got %v, want %v", tt.attempt, got, tt.expected)
		}
	}
}

// TestBackoff_JitterRange_Specific 测试特定抖动范围
func TestBackoff_JitterRange_Specific(t *testing.T) {
	base := time.Second
	max := 30 * time.Second
	jitter := 0.2 // ±20%

	// 运行多次验证抖动范围
	for i := 0; i < 100; i++ {
		b := New(base, max, jitter)
		delay := b.Next()

		minExpected := float64(base) * 0.8 // 1s * 0.8 = 0.8s
		maxExpected := float64(base) * 1.2 // 1s * 1.2 = 1.2s

		if float64(delay) < minExpected || float64(delay) > maxExpected {
			t.Errorf("第 %d 次: delay = %v, 期望范围 [%v, %v]",
				i, delay, time.Duration(minExpected), time.Duration(maxExpected))
		}
	}
}
