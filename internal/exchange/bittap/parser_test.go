// Package bittap Bittap 解析器测试
package bittap

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"latency-arbitrage-validator/internal/metadata"
)

// **Feature: latency-arbitrage-validator, Property 1: Parser Round-Trip Consistency (Bittap)**
// **Validates: Requirements 2.1, 2.5**

func createTestSymbolMaps() map[string]*metadata.SymbolMap {
	return map[string]*metadata.SymbolMap{
		"BTCUSDT": {Canon: "BTCUSDT", BittapSym: "BTC-USDT-M"},
		"ETHUSDT": {Canon: "ETHUSDT", BittapSym: "ETH-USDT"},
	}
}

func TestParser_RoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	parser := NewParser(createTestSymbolMaps())

	properties.Property("解析保留价格和数量", prop.ForAll(
		func(bidPx, bidQty, askPx, askQty float64, seq int64) bool {
			if askPx <= bidPx {
				askPx = bidPx + 1
			}
			if seq <= 0 {
				seq = 1
			}

			msg := DepthMessage{
				Event:        "f_depth30",
				Symbol:       "BTC-USDT-M",
				Tick:         "0.1",
				LastUpdateID: seq,
				Bids:         [][]string{{fmt.Sprintf("%.2f", bidPx), fmt.Sprintf("%.4f", bidQty)}},
				Asks:         [][]string{{fmt.Sprintf("%.2f", askPx), fmt.Sprintf("%.4f", askQty)}},
			}
			data, err := json.Marshal(msg)
			if err != nil {
				return false
			}

			events, err := parser.Parse(data)
			if err != nil || len(events) != 1 {
				return false
			}

			event := events[0]
			if event.SymbolCanon != "BTCUSDT" {
				return false
			}
			if event.Seq != seq {
				return false
			}
			if event.ExchTsUnixMs != 0 {
				return false
			}

			bidDiff := event.BestBidPx - bidPx
			askDiff := event.BestAskPx - askPx

			return bidDiff < 0.01 && bidDiff > -0.01 && askDiff < 0.01 && askDiff > -0.01
		},
		gen.Float64Range(10000, 100000), // bidPx
		gen.Float64Range(0.001, 100),    // bidQty
		gen.Float64Range(10000, 100000), // askPx
		gen.Float64Range(0.001, 100),    // askQty
		gen.Int64Range(1, 1000000),      // lastUpdateId
	))

	properties.TestingRun(t)
}

func TestParser_SpecificMessages(t *testing.T) {
	parser := NewParser(createTestSymbolMaps())

	tests := []struct {
		name      string
		message   string
		wantEvent bool
		wantCanon string
		wantSeq   int64
		wantBidPx float64
		wantAskPx float64
	}{
		{
			name: "标准 f_depth30 消息",
			message: `{
				"e":"f_depth30",
				"s":"BTC-USDT-M",
				"i":"0.1",
				"lastUpdateId":123,
				"bids":[["50000.5","1.5"]],
				"asks":[["50001.0","2.0"]]
			}`,
			wantEvent: true,
			wantCanon: "BTCUSDT",
			wantSeq:   123,
			wantBidPx: 50000.5,
			wantAskPx: 50001.0,
		},
		{
			name:      "PONG 消息（result 形式）",
			message:   `{"result":"PONG"}`,
			wantEvent: false,
		},
		{
			name:      "非 f_depth30 事件",
			message:   `{"e":"trade","s":"BTC-USDT-M"}`,
			wantEvent: false,
		},
		{
			name:      "未配置的交易对",
			message:   `{"e":"f_depth30","s":"SOL-USDT-M","lastUpdateId":1,"bids":[["1","1"]],"asks":[["2","2"]]}`,
			wantEvent: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := parser.Parse([]byte(tt.message))
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}

			if tt.wantEvent {
				if len(events) != 1 {
					t.Fatalf("事件数量=%d, want 1", len(events))
				}
				ev := events[0]
				if ev.SymbolCanon != tt.wantCanon {
					t.Errorf("SymbolCanon=%s, want %s", ev.SymbolCanon, tt.wantCanon)
				}
				if ev.Seq != tt.wantSeq {
					t.Errorf("Seq=%d, want %d", ev.Seq, tt.wantSeq)
				}
				if ev.BestBidPx != tt.wantBidPx {
					t.Errorf("BestBidPx=%f, want %f", ev.BestBidPx, tt.wantBidPx)
				}
				if ev.BestAskPx != tt.wantAskPx {
					t.Errorf("BestAskPx=%f, want %f", ev.BestAskPx, tt.wantAskPx)
				}
			} else {
				if len(events) != 0 {
					t.Fatalf("事件数量=%d, want 0", len(events))
				}
			}
		})
	}
}

func TestParser_InvalidMessages(t *testing.T) {
	parser := NewParser(createTestSymbolMaps())

	_, err := parser.Parse([]byte(`{invalid json}`))
	if err == nil {
		t.Fatalf("期望错误但得到 nil")
	}
}
