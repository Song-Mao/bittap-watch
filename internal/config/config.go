// Package config 负责加载和验证 YAML 配置文件。
// 提供应用程序所需的所有配置项，包括交易所连接、策略参数、手续费设置等。
package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 应用配置根结构
// 包含所有子模块的配置项
type Config struct {
	// App 应用基础配置
	App AppConfig `yaml:"app"`
	// Symbols 用户配置的交易对列表
	Symbols []SymbolConfig `yaml:"symbols"`
	// Metadata 元数据 API 配置
	Metadata MetadataConfig `yaml:"metadata"`
	// WS WebSocket 连接配置
	WS WSConfig `yaml:"ws"`
	// Fees 手续费配置
	Fees FeesConfig `yaml:"fees"`
	// Strategy 策略参数配置
	Strategy StrategyConfig `yaml:"strategy"`
	// Paper 影子成交配置
	Paper PaperConfig `yaml:"paper"`
	// Output 输出配置
	Output OutputConfig `yaml:"output"`
}

// AppConfig 应用基础配置
type AppConfig struct {
	// Name 应用名称，用于日志标识
	Name string `yaml:"name"`
	// LogLevel 日志级别: debug, info, warn, error
	LogLevel string `yaml:"log_level"`
}

// SymbolConfig 交易对配置
type SymbolConfig struct {
	// Input 用户输入的交易对格式，如 BTC-USDT
	Input string `yaml:"input"`
}

// MetadataConfig 元数据 API 配置
type MetadataConfig struct {
	// OKX OKX 合约元数据 API 地址
	OKX string `yaml:"okx"`
	// Binance Binance 合约元数据 API 地址
	Binance string `yaml:"binance"`
	// Bittap Bittap 合约元数据 API 地址
	Bittap string `yaml:"bittap"`
	// TimeoutMs HTTP 请求超时时间（毫秒）
	TimeoutMs int `yaml:"timeout_ms"`
}

// WSConfig WebSocket 连接配置
type WSConfig struct {
	// OKX OKX WebSocket 配置
	OKX ExchangeWSConfig `yaml:"okx"`
	// Binance Binance WebSocket 配置
	Binance ExchangeWSConfig `yaml:"binance"`
	// Bittap Bittap WebSocket 配置
	Bittap ExchangeWSConfig `yaml:"bittap"`
}

// ExchangeWSConfig 单个交易所的 WebSocket 配置
type ExchangeWSConfig struct {
	// URL WebSocket 连接地址
	URL string `yaml:"url"`
	// PingIntervalMs 心跳间隔（毫秒）
	PingIntervalMs int `yaml:"ping_interval_ms"`
	// PongTimeoutMs 心跳响应超时（毫秒）
	PongTimeoutMs int `yaml:"pong_timeout_ms"`
	// ReadTimeoutMs 读取超时（毫秒）
	ReadTimeoutMs int `yaml:"read_timeout_ms"`
}

// FeesConfig 手续费配置
type FeesConfig struct {
	// Bittap Bittap 交易所手续费配置（影子成交使用）
	Bittap FeeDetail `yaml:"bittap"`
}

// FeeDetail 手续费详情
type FeeDetail struct {
	// TakerRate Taker 手续费率（0-1）
	TakerRate float64 `yaml:"taker_rate"`
	// MakerRate Maker 手续费率（0-1）
	MakerRate float64 `yaml:"maker_rate"`
	// RebateRate 返佣比例（0-1）
	RebateRate float64 `yaml:"rebate_rate"`
}

// StrategyConfig 策略参数配置
type StrategyConfig struct {
	// ThetaEntryBps 入场阈值（基点），价差超过此值才触发信号
	ThetaEntryBps float64 `yaml:"theta_entry_bps"`
	// PersistMs 持续时间过滤（毫秒），价差需持续超过此时间
	PersistMs int `yaml:"persist_ms"`
	// MinDepthUSD 最小深度过滤（USD），Leader 前 5 档深度需超过此值
	MinDepthUSD float64 `yaml:"min_depth_usd"`
	// VolFilterEnabled 是否启用波动率过滤
	VolFilterEnabled bool `yaml:"vol_filter_enabled"`
	// VolThreshold 波动率阈值，1 分钟实现波动率超过此值跳过信号
	VolThreshold float64 `yaml:"vol_threshold"`
	// CooldownMs 止损冷却时间（毫秒）
	CooldownMs int `yaml:"cooldown_ms"`
}

// PaperConfig 影子成交配置
type PaperConfig struct {
	// TPRatio 止盈比例，价差收敛到 (1-r_tp)*入场价差 时止盈
	TPRatio float64 `yaml:"tp_ratio"`
	// SLRatio 止损比例，价差发散到 (1+r_sl)*入场价差 时止损
	SLRatio float64 `yaml:"sl_ratio"`
	// MaxHoldMs 最大持仓时间（毫秒），超时强制平仓
	MaxHoldMs int `yaml:"max_hold_ms"`
	// SlippageBps 滑点（基点），影子成交时额外扣除
	SlippageBps float64 `yaml:"slippage_bps"`
}

// OutputConfig 输出配置
type OutputConfig struct {
	// Dir 输出目录
	Dir string `yaml:"dir"`
	// SignalsEnabled 是否输出信号文件
	SignalsEnabled bool `yaml:"signals_enabled"`
	// PaperTradesEnabled 是否输出影子成交文件
	PaperTradesEnabled bool `yaml:"paper_trades_enabled"`
	// MetricsEnabled 是否输出指标文件
	MetricsEnabled bool `yaml:"metrics_enabled"`
	// MetricsIntervalMs 指标输出间隔（毫秒）
	MetricsIntervalMs int `yaml:"metrics_interval_ms"`
	// BufferSize 异步写入缓冲区大小
	BufferSize int `yaml:"buffer_size"`
}

// Load 从文件加载配置并验证
// 参数 path: 配置文件路径
// 返回: 解析后的配置对象，若失败则返回错误
func Load(path string) (*Config, error) {
	// 读取配置文件
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析 YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	cfg.setDefaults()

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &cfg, nil
}

// setDefaults 设置配置默认值
func (c *Config) setDefaults() {
	// 应用默认值
	if c.App.Name == "" {
		c.App.Name = "latency-arbitrage-validator"
	}
	if c.App.LogLevel == "" {
		c.App.LogLevel = "info"
	}

	// 元数据 API 默认超时
	if c.Metadata.TimeoutMs == 0 {
		c.Metadata.TimeoutMs = 10000 // 10 秒
	}

	// WebSocket 默认配置
	if c.WS.OKX.PingIntervalMs == 0 {
		c.WS.OKX.PingIntervalMs = 25000 // 25 秒
	}
	if c.WS.OKX.PongTimeoutMs == 0 {
		c.WS.OKX.PongTimeoutMs = 10000 // 10 秒
	}
	if c.WS.Bittap.PingIntervalMs == 0 {
		c.WS.Bittap.PingIntervalMs = 18000 // 18 秒
	}
	if c.WS.Binance.ReadTimeoutMs == 0 {
		c.WS.Binance.ReadTimeoutMs = 30000 // 30 秒
	}

	// 策略默认值
	if c.Strategy.PersistMs == 0 {
		c.Strategy.PersistMs = 100 // 100 毫秒
	}
	if c.Strategy.CooldownMs == 0 {
		c.Strategy.CooldownMs = 3000 // 3 秒
	}

	// 影子成交默认值
	if c.Paper.MaxHoldMs == 0 {
		c.Paper.MaxHoldMs = 60000 // 60 秒
	}

	// 输出默认值
	if c.Output.Dir == "" {
		c.Output.Dir = "./output"
	}
	if c.Output.MetricsIntervalMs == 0 {
		c.Output.MetricsIntervalMs = 10000 // 10 秒
	}
	if c.Output.BufferSize == 0 {
		c.Output.BufferSize = 1000
	}
}

// Validate 验证配置合法性
// 检查所有必填项和数值范围
// 返回: 若配置无效则返回描述性错误
func (c *Config) Validate() error {
	var errs []string

	// 验证交易对配置
	if len(c.Symbols) == 0 {
		errs = append(errs, "symbols: 至少需要配置一个交易对")
	}
	for i, sym := range c.Symbols {
		if sym.Input == "" {
			errs = append(errs, fmt.Sprintf("symbols[%d].input: 交易对不能为空", i))
		}
	}

	// 验证元数据 API 配置
	if c.Metadata.OKX == "" {
		errs = append(errs, "metadata.okx: OKX 元数据 API 地址不能为空")
	}
	if c.Metadata.Binance == "" {
		errs = append(errs, "metadata.binance: Binance 元数据 API 地址不能为空")
	}
	if c.Metadata.Bittap == "" {
		errs = append(errs, "metadata.bittap: Bittap 元数据 API 地址不能为空")
	}

	// 验证 WebSocket 配置
	if c.WS.OKX.URL == "" {
		errs = append(errs, "ws.okx.url: OKX WebSocket 地址不能为空")
	}
	if c.WS.Binance.URL == "" {
		errs = append(errs, "ws.binance.url: Binance WebSocket 地址不能为空")
	}
	if c.WS.Bittap.URL == "" {
		errs = append(errs, "ws.bittap.url: Bittap WebSocket 地址不能为空")
	}

	// 验证手续费配置（范围 0-1）
	if err := validateFeeRate(c.Fees.Bittap.TakerRate, "fees.bittap.taker_rate"); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validateFeeRate(c.Fees.Bittap.MakerRate, "fees.bittap.maker_rate"); err != nil {
		errs = append(errs, err.Error())
	}
	if err := validateFeeRate(c.Fees.Bittap.RebateRate, "fees.bittap.rebate_rate"); err != nil {
		errs = append(errs, err.Error())
	}

	// 验证策略参数
	if c.Strategy.ThetaEntryBps <= 0 {
		errs = append(errs, "strategy.theta_entry_bps: 入场阈值必须为正数")
	}
	if c.Strategy.PersistMs <= 0 {
		errs = append(errs, "strategy.persist_ms: 持续时间必须为正数")
	}
	if c.Strategy.CooldownMs < 0 {
		errs = append(errs, "strategy.cooldown_ms: 冷却时间不能为负数")
	}

	// 验证影子成交参数
	if c.Paper.TPRatio < 0 || c.Paper.TPRatio > 1 {
		errs = append(errs, "paper.tp_ratio: 止盈比例必须在 0-1 之间")
	}
	if c.Paper.SLRatio < 0 {
		errs = append(errs, "paper.sl_ratio: 止损比例不能为负数")
	}
	if c.Paper.MaxHoldMs <= 0 {
		errs = append(errs, "paper.max_hold_ms: 最大持仓时间必须为正数")
	}
	if c.Paper.SlippageBps < 0 {
		errs = append(errs, "paper.slippage_bps: 滑点不能为负数")
	}

	// 验证日志级别
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[strings.ToLower(c.App.LogLevel)] {
		errs = append(errs, fmt.Sprintf("app.log_level: 无效的日志级别 '%s'，有效值: debug, info, warn, error", c.App.LogLevel))
	}

	if len(errs) > 0 {
		return fmt.Errorf("配置验证错误:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// validateFeeRate 验证手续费率范围
// 参数 rate: 费率值
// 参数 field: 字段名称，用于错误消息
// 返回: 若费率无效则返回错误
func validateFeeRate(rate float64, field string) error {
	if rate < 0 || rate > 1 {
		return fmt.Errorf("%s: 费率必须在 0-1 之间，当前值: %f", field, rate)
	}
	return nil
}

// GetSymbolInputs 获取所有配置的交易对输入
// 返回: 交易对输入字符串列表
func (c *Config) GetSymbolInputs() []string {
	inputs := make([]string, len(c.Symbols))
	for i, sym := range c.Symbols {
		inputs[i] = sym.Input
	}
	return inputs
}

// EffectiveTakerFee 计算有效 Taker 手续费（考虑返佣）
// 返回: 有效手续费率
func (f *FeeDetail) EffectiveTakerFee() float64 {
	return f.TakerRate * (1 - f.RebateRate)
}

// EffectiveMakerFee 计算有效 Maker 手续费（考虑返佣）
// 返回: 有效手续费率
func (f *FeeDetail) EffectiveMakerFee() float64 {
	return f.MakerRate * (1 - f.RebateRate)
}
