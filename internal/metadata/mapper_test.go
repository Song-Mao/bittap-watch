// Package metadata 元数据模块测试
package metadata

import (
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: latency-arbitrage-validator, Property 2: Symbol Normalization Consistency**
// **Validates: Requirements 2.6, 3.2**

// TestNormalizeSymbol_Consistency 测试 Symbol 标准化一致性
// 属性: 不同格式的同一交易对应该标准化为相同的 Canon
func TestNormalizeSymbol_Consistency(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 使用固定的币种列表进行测试
	coins := []string{"BTC", "ETH", "SOL", "DOGE", "XRP", "ADA", "DOT", "LINK", "UNI", "AVAX"}

	// 属性: 带分隔符和不带分隔符的格式应该标准化为相同结果
	properties.Property("分隔符不影响标准化结果", prop.ForAll(
		func(baseIdx int, quoteIdx int) bool {
			base := coins[baseIdx%len(coins)]
			quote := coins[quoteIdx%len(coins)]

			// 不同格式
			withDash := base + "-" + quote
			withUnderscore := base + "_" + quote
			withSlash := base + "/" + quote
			noDash := base + quote

			// 标准化后应该相同
			canon1 := normalizeSymbol(withDash)
			canon2 := normalizeSymbol(withUnderscore)
			canon3 := normalizeSymbol(withSlash)
			canon4 := normalizeSymbol(noDash)

			return canon1 == canon2 && canon2 == canon3 && canon3 == canon4
		},
		gen.IntRange(0, 9),
		gen.IntRange(0, 9),
	))

	// 属性: 大小写不影响标准化结果
	properties.Property("大小写不影响标准化结果", prop.ForAll(
		func(baseIdx int, quoteIdx int) bool {
			base := coins[baseIdx%len(coins)]
			quote := coins[quoteIdx%len(coins)]

			upper := base + "-" + quote
			lower := strings.ToLower(base) + "-" + strings.ToLower(quote)
			mixed := strings.ToLower(base) + "-" + quote

			canon1 := normalizeSymbol(upper)
			canon2 := normalizeSymbol(lower)
			canon3 := normalizeSymbol(mixed)

			return canon1 == canon2 && canon2 == canon3
		},
		gen.IntRange(0, 9),
		gen.IntRange(0, 9),
	))

	properties.TestingRun(t)
}

// TestNormalizeSymbol_Idempotent 测试标准化幂等性
// 属性: 对已标准化的结果再次标准化应该得到相同结果
func TestNormalizeSymbol_Idempotent(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 使用固定的交易对格式进行测试
	formats := []string{
		"BTC-USDT", "ETH-USDT", "SOL-USDT",
		"btc-usdt", "eth_usdt", "sol/usdt",
		"BTCUSDT", "ETHUSDT", "SOLUSDT",
		"BTC-USDT-SWAP", "ETH-USDT-M",
	}

	properties.Property("标准化是幂等的", prop.ForAll(
		func(idx int) bool {
			input := formats[idx%len(formats)]
			canon1 := normalizeSymbol(input)
			canon2 := normalizeSymbol(canon1)
			return canon1 == canon2
		},
		gen.IntRange(0, len(formats)-1),
	))

	properties.TestingRun(t)
}

// TestNormalizeSymbol_SpecificCases 测试特定交易对格式
func TestNormalizeSymbol_SpecificCases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTC-USDT", "BTCUSDT"},
		{"btc-usdt", "BTCUSDT"},
		{"BTC_USDT", "BTCUSDT"},
		{"BTC/USDT", "BTCUSDT"},
		{"BTCUSDT", "BTCUSDT"},
		{"ETH-USDT-SWAP", "ETHUSDT"},
		{"BTC-USDT-M", "BTCUSDT"},
		{"sol-usdt", "SOLUSDT"},
	}

	for _, tt := range tests {
		got := normalizeSymbol(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeSymbol(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// TestOKXInstrument_IsUSDTLinearSwap 测试 OKX 合约类型判断
func TestOKXInstrument_IsUSDTLinearSwap(t *testing.T) {
	tests := []struct {
		name     string
		inst     OKXInstrument
		expected bool
	}{
		{
			name: "USDT 正向永续",
			inst: OKXInstrument{
				InstType:  "SWAP",
				CtType:    "linear",
				SettleCcy: "USDT",
			},
			expected: true,
		},
		{
			name: "USD 反向永续",
			inst: OKXInstrument{
				InstType:  "SWAP",
				CtType:    "inverse",
				SettleCcy: "BTC",
			},
			expected: false,
		},
		{
			name: "交割合约",
			inst: OKXInstrument{
				InstType:  "FUTURES",
				CtType:    "linear",
				SettleCcy: "USDT",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.inst.IsUSDTLinearSwap()
			if got != tt.expected {
				t.Errorf("IsUSDTLinearSwap() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestBinanceSymbol_IsUSDTPerpetual 测试 Binance 合约类型判断
func TestBinanceSymbol_IsUSDTPerpetual(t *testing.T) {
	tests := []struct {
		name     string
		sym      BinanceSymbol
		expected bool
	}{
		{
			name: "USDT 永续",
			sym: BinanceSymbol{
				ContractType: "PERPETUAL",
				QuoteAsset:   "USDT",
				Status:       "TRADING",
			},
			expected: true,
		},
		{
			name: "当季合约",
			sym: BinanceSymbol{
				ContractType: "CURRENT_QUARTER",
				QuoteAsset:   "USDT",
				Status:       "TRADING",
			},
			expected: false,
		},
		{
			name: "非 USDT 永续",
			sym: BinanceSymbol{
				ContractType: "PERPETUAL",
				QuoteAsset:   "BUSD",
				Status:       "TRADING",
			},
			expected: false,
		},
		{
			name: "非交易状态",
			sym: BinanceSymbol{
				ContractType: "PERPETUAL",
				QuoteAsset:   "USDT",
				Status:       "BREAK",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sym.IsUSDTPerpetual()
			if got != tt.expected {
				t.Errorf("IsUSDTPerpetual() = %v, want %v", got, tt.expected)
			}
		})
	}
}
