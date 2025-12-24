// Package okx OKX 解析器测试
package okx

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"latency-arbitrage-validator/internal/metadata"
)

// **Feature: latency-arbitrage-validator, Property 1: Parser Round-Trip Consistency (OKX)**
// **Validates: Requirements 2.1, 2.3**

// 创建测试用的 Symbol 映射
func createTestSymbolMaps() map[string]*metadata.SymbolMap {
	return map[string]*metadata.SymbolMap{
		"BTCUSDT": {
			Canon:     "BTCUSDT",
			OKXInstId: "BTC-USDT-SWAP",
		},
		"ETHUSDT": {
			Canon:     "ETHUSDT",
			OKXInstId: "ETH-USDT-SWAP",
		},
	}
}

// TestParser_RoundTrip 测试解析器往返一致性
// 属性: 解析后的 BookEvent 应保留原始价格和数量信息
func TestParser_RoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	symbolMaps := createTestSymbolMaps()
	parser := NewParser(symbolMaps)

	// 属性: 解析后的价格和数量应与原始数据一致
	properties.Property("解析保留价格和数量", prop.ForAll(
		func(bidPx, bidQty, askPx, askQty float64, ts int64, seqId int64) bool {
			// 确保 askPx > bidPx
			if askPx <= bidPx {
				askPx = bidPx + 1
			}

			// 构造 OKX books5 消息
			msg := Books5Message{
				Arg: SubscribeArg{
					Channel: "books5",
					InstId:  "BTC-USDT-SWAP",
				},
				Data: []Books5Data{
					{
						InstId: "BTC-USDT-SWAP",
						Bids:   [][]string{{fmt.Sprintf("%.2f", bidPx), fmt.Sprintf("%.4f", bidQty), "0", "1"}},
						Asks:   [][]string{{fmt.Sprintf("%.2f", askPx), fmt.Sprintf("%.4f", askQty), "0", "1"}},
						Ts:     fmt.Sprintf("%d", ts),
						SeqId:  seqId,
					},
				},
			}

			// 序列化
			data, err := json.Marshal(msg)
			if err != nil {
				return false
			}

			// 解析
			events, err := parser.Parse(data)
			if err != nil {
				return false
			}

			if len(events) != 1 {
				return false
			}

			event := events[0]

			// 验证价格和数量（允许浮点数精度误差）
			bidPxDiff := event.BestBidPx - bidPx
			askPxDiff := event.BestAskPx - askPx

			return bidPxDiff < 0.01 && bidPxDiff > -0.01 &&
				askPxDiff < 0.01 && askPxDiff > -0.01 &&
				event.ExchTsUnixMs == ts &&
				event.Seq == seqId &&
				event.SymbolCanon == "BTCUSDT"
		},
		gen.Float64Range(10000, 100000),              // bidPx
		gen.Float64Range(0.001, 100),                 // bidQty
		gen.Float64Range(10000, 100000),              // askPx
		gen.Float64Range(0.001, 100),                 // askQty
		gen.Int64Range(1700000000000, 1800000000000), // ts
		gen.Int64Range(1, 1000000),                   // seqId
	))

	properties.TestingRun(t)
}

// TestParser_SpecificMessages 测试特定消息格式
func TestParser_SpecificMessages(t *testing.T) {
	symbolMaps := createTestSymbolMaps()
	parser := NewParser(symbolMaps)

	tests := []struct {
		name       string
		message    string
		wantEvents int
		wantCanon  string
		wantBidPx  float64
		wantAskPx  float64
		wantTs     int64
		wantSeq    int64
	}{
		{
			name: "标准 books5 消息",
			message: `{
				"arg": {"channel": "books5", "instId": "BTC-USDT-SWAP"},
				"data": [{
					"instId": "BTC-USDT-SWAP",
					"bids": [["50000.5", "1.5", "0", "3"]],
					"asks": [["50001.0", "2.0", "0", "5"]],
					"ts": "1700000000000",
					"seqId": 12345
				}]
			}`,
			wantEvents: 1,
			wantCanon:  "BTCUSDT",
			wantBidPx:  50000.5,
			wantAskPx:  50001.0,
			wantTs:     1700000000000,
			wantSeq:    12345,
		},
		{
			name: "ETH 交易对",
			message: `{
				"arg": {"channel": "books5", "instId": "ETH-USDT-SWAP"},
				"data": [{
					"instId": "ETH-USDT-SWAP",
					"bids": [["3000.00", "10.0", "0", "2"]],
					"asks": [["3000.50", "5.0", "0", "1"]],
					"ts": "1700000001000",
					"seqId": 67890
				}]
			}`,
			wantEvents: 1,
			wantCanon:  "ETHUSDT",
			wantBidPx:  3000.00,
			wantAskPx:  3000.50,
			wantTs:     1700000001000,
			wantSeq:    67890,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events, err := parser.Parse([]byte(tt.message))
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}

			if len(events) != tt.wantEvents {
				t.Fatalf("事件数量 = %d, want %d", len(events), tt.wantEvents)
			}

			if tt.wantEvents > 0 {
				event := events[0]
				if event.SymbolCanon != tt.wantCanon {
					t.Errorf("SymbolCanon = %s, want %s", event.SymbolCanon, tt.wantCanon)
				}
				if event.BestBidPx != tt.wantBidPx {
					t.Errorf("BestBidPx = %f, want %f", event.BestBidPx, tt.wantBidPx)
				}
				if event.BestAskPx != tt.wantAskPx {
					t.Errorf("BestAskPx = %f, want %f", event.BestAskPx, tt.wantAskPx)
				}
				if event.ExchTsUnixMs != tt.wantTs {
					t.Errorf("ExchTsUnixMs = %d, want %d", event.ExchTsUnixMs, tt.wantTs)
				}
				if event.Seq != tt.wantSeq {
					t.Errorf("Seq = %d, want %d", event.Seq, tt.wantSeq)
				}
			}
		})
	}
}

// TestParser_InvalidMessages 测试无效消息处理
func TestParser_InvalidMessages(t *testing.T) {
	symbolMaps := createTestSymbolMaps()
	parser := NewParser(symbolMaps)

	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{
			name:    "无效 JSON",
			message: `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "非 books5 频道",
			message: `{"arg": {"channel": "trades", "instId": "BTC-USDT-SWAP"}, "data": []}`,
			wantErr: false, // 应该忽略，不报错
		},
		{
			name:    "未配置的交易对",
			message: `{"arg": {"channel": "books5", "instId": "SOL-USDT-SWAP"}, "data": [{"instId": "SOL-USDT-SWAP", "bids": [], "asks": [], "ts": "0", "seqId": 0}]}`,
			wantErr: false, // 应该忽略，不报错
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parser.Parse([]byte(tt.message))
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestIsPong 测试 pong 响应判断
func TestIsPong(t *testing.T) {
	tests := []struct {
		data string
		want bool
	}{
		{"pong", true},
		{"ping", false},
		{`{"event": "subscribe"}`, false},
	}

	for _, tt := range tests {
		got := IsPong([]byte(tt.data))
		if got != tt.want {
			t.Errorf("IsPong(%q) = %v, want %v", tt.data, got, tt.want)
		}
	}
}

// TestIsSubscribeResponse 测试订阅响应判断
func TestIsSubscribeResponse(t *testing.T) {
	tests := []struct {
		data string
		want bool
	}{
		{`{"event": "subscribe", "arg": {"channel": "books5"}}`, true},
		{`{"event": "error", "code": "1", "msg": "error"}`, true},
		{`{"arg": {"channel": "books5"}, "data": []}`, false},
		{`pong`, false},
	}

	for _, tt := range tests {
		got := IsSubscribeResponse([]byte(tt.data))
		if got != tt.want {
			t.Errorf("IsSubscribeResponse(%q) = %v, want %v", tt.data, got, tt.want)
		}
	}
}
