// Package config 配置模块测试
package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: latency-arbitrage-validator, Property 20: Config Validation Correctness**
// **Validates: Requirements 9.2, 9.3, 9.4, 9.5**

// TestConfigValidation_FeeRateRange 测试手续费率范围验证
// 属性: 费率在 [0, 1] 范围外应验证失败
func TestConfigValidation_FeeRateRange(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: 费率 < 0 应验证失败
	properties.Property("费率小于0应验证失败", prop.ForAll(
		func(rate float64) bool {
			cfg := createValidConfig()
			cfg.Fees.Bittap.TakerRate = rate
			err := cfg.Validate()
			return err != nil // 应该返回错误
		},
		gen.Float64Range(-1000, -0.0001), // 负数费率
	))

	// 属性: 费率 > 1 应验证失败
	properties.Property("费率大于1应验证失败", prop.ForAll(
		func(rate float64) bool {
			cfg := createValidConfig()
			cfg.Fees.Bittap.TakerRate = rate
			err := cfg.Validate()
			return err != nil // 应该返回错误
		},
		gen.Float64Range(1.0001, 1000), // 超过1的费率
	))

	// 属性: 费率在 [0, 1] 范围内应验证通过（仅费率部分）
	properties.Property("费率在有效范围内应通过验证", prop.ForAll(
		func(rate float64) bool {
			cfg := createValidConfig()
			cfg.Fees.Bittap.TakerRate = rate
			cfg.Fees.Bittap.MakerRate = rate
			cfg.Fees.Bittap.RebateRate = rate
			err := cfg.Validate()
			return err == nil // 应该通过验证
		},
		gen.Float64Range(0, 1), // 有效费率范围
	))

	properties.TestingRun(t)
}

// TestConfigValidation_StrategyParams 测试策略参数验证
// 属性: θ_entry、persist_ms、max_hold_ms 必须为正数
func TestConfigValidation_StrategyParams(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: θ_entry <= 0 应验证失败
	properties.Property("入场阈值非正数应验证失败", prop.ForAll(
		func(theta float64) bool {
			cfg := createValidConfig()
			cfg.Strategy.ThetaEntryBps = theta
			err := cfg.Validate()
			return err != nil
		},
		gen.Float64Range(-1000, 0), // 非正数
	))

	// 属性: θ_entry > 0 应验证通过
	properties.Property("入场阈值为正数应通过验证", prop.ForAll(
		func(theta float64) bool {
			cfg := createValidConfig()
			cfg.Strategy.ThetaEntryBps = theta
			err := cfg.Validate()
			return err == nil
		},
		gen.Float64Range(0.0001, 1000), // 正数
	))

	// 属性: persist_ms <= 0 应验证失败
	properties.Property("持续时间非正数应验证失败", prop.ForAll(
		func(persist int) bool {
			cfg := createValidConfig()
			cfg.Strategy.PersistMs = persist
			err := cfg.Validate()
			return err != nil
		},
		gen.IntRange(-1000, 0), // 非正数
	))

	// 属性: max_hold_ms <= 0 应验证失败
	properties.Property("最大持仓时间非正数应验证失败", prop.ForAll(
		func(maxHold int) bool {
			cfg := createValidConfig()
			cfg.Paper.MaxHoldMs = maxHold
			err := cfg.Validate()
			return err != nil
		},
		gen.IntRange(-1000, 0), // 非正数
	))

	properties.TestingRun(t)
}

// TestConfigValidation_Symbols 测试交易对配置验证
// 属性: 空交易对列表应验证失败
func TestConfigValidation_Symbols(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: 空交易对列表应验证失败
	properties.Property("空交易对列表应验证失败", prop.ForAll(
		func(_ int) bool {
			cfg := createValidConfig()
			cfg.Symbols = []SymbolConfig{} // 空列表
			err := cfg.Validate()
			return err != nil
		},
		gen.Int(), // 占位生成器
	))

	// 属性: 交易对输入为空字符串应验证失败
	properties.Property("空交易对输入应验证失败", prop.ForAll(
		func(_ int) bool {
			cfg := createValidConfig()
			cfg.Symbols = []SymbolConfig{{Input: ""}}
			err := cfg.Validate()
			return err != nil
		},
		gen.Int(),
	))

	// 属性: 有效交易对应通过验证
	properties.Property("有效交易对应通过验证", prop.ForAll(
		func(symbol string) bool {
			if symbol == "" {
				return true // 跳过空字符串
			}
			cfg := createValidConfig()
			cfg.Symbols = []SymbolConfig{{Input: symbol}}
			err := cfg.Validate()
			return err == nil
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

// TestConfigValidation_ValidConfig 测试有效配置应通过验证
func TestConfigValidation_ValidConfig(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: 所有参数在有效范围内的配置应通过验证
	properties.Property("有效配置应通过验证", prop.ForAll(
		func(theta float64, persist int, maxHold int, feeRate float64) bool {
			cfg := createValidConfig()
			cfg.Strategy.ThetaEntryBps = theta
			cfg.Strategy.PersistMs = persist
			cfg.Paper.MaxHoldMs = maxHold
			cfg.Fees.Bittap.TakerRate = feeRate
			cfg.Fees.Bittap.MakerRate = feeRate
			cfg.Fees.Bittap.RebateRate = feeRate
			err := cfg.Validate()
			return err == nil
		},
		gen.Float64Range(0.0001, 1000), // 有效 θ_entry
		gen.IntRange(1, 10000),         // 有效 persist_ms
		gen.IntRange(1, 1000000),       // 有效 max_hold_ms
		gen.Float64Range(0, 1),         // 有效费率
	))

	properties.TestingRun(t)
}

// TestConfigValidation_PaperParams 测试影子成交参数验证
func TestConfigValidation_PaperParams(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// 属性: TP 比例超出 [0, 1] 应验证失败
	properties.Property("止盈比例超出范围应验证失败", prop.ForAll(
		func(ratio float64) bool {
			cfg := createValidConfig()
			cfg.Paper.TPRatio = ratio
			err := cfg.Validate()
			return err != nil
		},
		gen.OneGenOf(
			gen.Float64Range(-1000, -0.0001), // 负数
			gen.Float64Range(1.0001, 1000),   // 超过1
		),
	))

	// 属性: SL 比例为负数应验证失败
	properties.Property("止损比例为负数应验证失败", prop.ForAll(
		func(ratio float64) bool {
			cfg := createValidConfig()
			cfg.Paper.SLRatio = ratio
			err := cfg.Validate()
			return err != nil
		},
		gen.Float64Range(-1000, -0.0001),
	))

	// 属性: 滑点为负数应验证失败
	properties.Property("滑点为负数应验证失败", prop.ForAll(
		func(slippage float64) bool {
			cfg := createValidConfig()
			cfg.Paper.SlippageBps = slippage
			err := cfg.Validate()
			return err != nil
		},
		gen.Float64Range(-1000, -0.0001),
	))

	properties.TestingRun(t)
}

// createValidConfig 创建一个有效的配置用于测试
func createValidConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:     "test",
			LogLevel: "info",
		},
		Symbols: []SymbolConfig{
			{Input: "BTC-USDT"},
		},
		Metadata: MetadataConfig{
			OKX:       "https://www.okx.com/api/v5/public/instruments",
			Binance:   "https://fapi.binance.com/fapi/v1/exchangeInfo",
			Bittap:    "https://api.bittap.com/api/v1/exchangeInfo",
			TimeoutMs: 10000,
		},
		WS: WSConfig{
			OKX: ExchangeWSConfig{
				URL:            "wss://ws.okx.com:8443/ws/v5/public",
				PingIntervalMs: 25000,
				PongTimeoutMs:  10000,
			},
			Binance: ExchangeWSConfig{
				URL:           "wss://fstream.binance.com/ws",
				ReadTimeoutMs: 30000,
			},
			Bittap: ExchangeWSConfig{
				URL:            "wss://stream.bittap.com/endpoint",
				PingIntervalMs: 18000,
			},
		},
		Fees: FeesConfig{
			Bittap: FeeDetail{
				TakerRate:  0.0004,
				MakerRate:  0.0002,
				RebateRate: 0.1,
			},
		},
		Strategy: StrategyConfig{
			ThetaEntryBps:    5.0,
			PersistMs:        100,
			MinDepthUSD:      10000,
			VolFilterEnabled: false,
			VolThreshold:     0.02,
			CooldownMs:       3000,
		},
		Paper: PaperConfig{
			TPRatio:     0.5,
			SLRatio:     1.0,
			MaxHoldMs:   60000,
			SlippageBps: 1.0,
		},
		Output: OutputConfig{
			Dir:                "./output",
			SignalsEnabled:     true,
			PaperTradesEnabled: true,
			MetricsEnabled:     true,
			MetricsIntervalMs:  10000,
			BufferSize:         1000,
		},
	}
}

// TestLoad_ValidFile 测试从有效文件加载配置
func TestLoad_ValidFile(t *testing.T) {
	// 创建临时配置文件
	content := `
app:
  name: test-validator
  log_level: info

symbols:
  - input: BTC-USDT
  - input: ETH-USDT

metadata:
  okx: https://www.okx.com/api/v5/public/instruments
  binance: https://fapi.binance.com/fapi/v1/exchangeInfo
  bittap: https://api.bittap.com/api/v1/exchangeInfo
  timeout_ms: 10000

ws:
  okx:
    url: wss://ws.okx.com:8443/ws/v5/public
    ping_interval_ms: 25000
    pong_timeout_ms: 10000
  binance:
    url: wss://fstream.binance.com/ws
    read_timeout_ms: 30000
  bittap:
    url: wss://stream.bittap.com/endpoint
    ping_interval_ms: 18000

fees:
  bittap:
    taker_rate: 0.0004
    maker_rate: 0.0002
    rebate_rate: 0.1

strategy:
  theta_entry_bps: 5.0
  persist_ms: 100
  min_depth_usd: 10000
  vol_filter_enabled: false
  vol_threshold: 0.02
  cooldown_ms: 3000

paper:
  tp_ratio: 0.5
  sl_ratio: 1.0
  max_hold_ms: 60000
  slippage_bps: 1.0

output:
  dir: ./output
  signals_enabled: true
  paper_trades_enabled: true
  metrics_enabled: true
  metrics_interval_ms: 10000
  buffer_size: 1000
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}

	// 加载配置
	cfg, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 验证加载的值
	if cfg.App.Name != "test-validator" {
		t.Errorf("App.Name = %s, want test-validator", cfg.App.Name)
	}
	if len(cfg.Symbols) != 2 {
		t.Errorf("len(Symbols) = %d, want 2", len(cfg.Symbols))
	}
	if cfg.Strategy.ThetaEntryBps != 5.0 {
		t.Errorf("Strategy.ThetaEntryBps = %f, want 5.0", cfg.Strategy.ThetaEntryBps)
	}
}

// TestLoad_InvalidFile 测试加载无效文件
func TestLoad_InvalidFile(t *testing.T) {
	// 测试不存在的文件
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("加载不存在的文件应返回错误")
	}
}

// TestLoad_InvalidYAML 测试加载无效 YAML
func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(tmpFile, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("加载无效 YAML 应返回错误")
	}
}

// TestEffectiveFee 测试有效手续费计算
func TestEffectiveFee(t *testing.T) {
	fee := FeeDetail{
		TakerRate:  0.0004,
		MakerRate:  0.0002,
		RebateRate: 0.25, // 25% 返佣
	}

	// 有效 Taker 费率 = 0.0004 * (1 - 0.25) = 0.0003
	expectedTaker := 0.0003
	gotTaker := fee.EffectiveTakerFee()
	// 使用容差比较浮点数
	if diff := gotTaker - expectedTaker; diff > 1e-10 || diff < -1e-10 {
		t.Errorf("EffectiveTakerFee() = %f, want %f", gotTaker, expectedTaker)
	}

	// 有效 Maker 费率 = 0.0002 * (1 - 0.25) = 0.00015
	expectedMaker := 0.00015
	gotMaker := fee.EffectiveMakerFee()
	if diff := gotMaker - expectedMaker; diff > 1e-10 || diff < -1e-10 {
		t.Errorf("EffectiveMakerFee() = %f, want %f", gotMaker, expectedMaker)
	}
}

// TestGetSymbolInputs 测试获取交易对输入列表
func TestGetSymbolInputs(t *testing.T) {
	cfg := &Config{
		Symbols: []SymbolConfig{
			{Input: "BTC-USDT"},
			{Input: "ETH-USDT"},
			{Input: "SOL-USDT"},
		},
	}

	inputs := cfg.GetSymbolInputs()
	if len(inputs) != 3 {
		t.Errorf("len(inputs) = %d, want 3", len(inputs))
	}
	if inputs[0] != "BTC-USDT" {
		t.Errorf("inputs[0] = %s, want BTC-USDT", inputs[0])
	}
}
