// Package ev EV 计算器测试
package ev

import (
	"math"
	"testing"
	"time"

	"latency-arbitrage-validator/internal/core/model"
)

func TestCalculator_Empty(t *testing.T) {
	c := NewCalculator(10)
	stats := c.Stats()
	if stats.Count != 0 {
		t.Fatalf("Count=%d, want 0", stats.Count)
	}
	if stats.EV != 0 {
		t.Fatalf("EV=%f, want 0", stats.EV)
	}
}

func TestCalculator_EVFormula(t *testing.T) {
	c := NewCalculator(100)

	// 2 赢 1 输；fee=2bps
	c.Add(&model.Position{Closed: true, NetPnLBps: 5, GrossPnLBps: 10, FeeBps: 2, ExitTime: time.Now()})
	c.Add(&model.Position{Closed: true, NetPnLBps: 10, GrossPnLBps: 20, FeeBps: 2, ExitTime: time.Now()})
	c.Add(&model.Position{Closed: true, NetPnLBps: -17, GrossPnLBps: -15, FeeBps: 2, ExitTime: time.Now()})

	stats := c.Stats()
	if stats.Count != 3 {
		t.Fatalf("Count=%d, want 3", stats.Count)
	}
	if stats.WinCount != 2 || stats.LossCount != 1 {
		t.Fatalf("WinCount=%d LossCount=%d, want 2/1", stats.WinCount, stats.LossCount)
	}

	// p=2/3, R=15, L=15, f=2 => EV=3
	if math.Abs(stats.EV-3.0) > 1e-9 {
		t.Fatalf("EV=%f, want 3", stats.EV)
	}
	if math.Abs(stats.PRequired-17.0/30.0) > 1e-9 {
		t.Fatalf("PRequired=%f, want %f", stats.PRequired, 17.0/30.0)
	}
}

func TestCalculator_RollingWindow(t *testing.T) {
	c := NewCalculator(2)

	c.Add(&model.Position{Closed: true, NetPnLBps: 1, GrossPnLBps: 10, FeeBps: 2})
	c.Add(&model.Position{Closed: true, NetPnLBps: -1, GrossPnLBps: -10, FeeBps: 2})
	c.Add(&model.Position{Closed: true, NetPnLBps: 1, GrossPnLBps: 20, FeeBps: 2})

	stats := c.Stats()
	if stats.Count != 2 {
		t.Fatalf("Count=%d, want 2", stats.Count)
	}
	// 窗口内应包含：loss(-10) 与 win(20)
	if stats.WinCount != 1 || stats.LossCount != 1 {
		t.Fatalf("WinCount=%d LossCount=%d, want 1/1", stats.WinCount, stats.LossCount)
	}
	if math.Abs(stats.AvgProfit-20.0) > 1e-9 {
		t.Fatalf("AvgProfit=%f, want 20", stats.AvgProfit)
	}
	if math.Abs(stats.AvgLoss-10.0) > 1e-9 {
		t.Fatalf("AvgLoss=%f, want 10", stats.AvgLoss)
	}
}
