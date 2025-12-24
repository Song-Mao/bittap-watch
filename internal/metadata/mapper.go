// Package metadata 负责从交易所获取合约元数据并构建 symbol 映射。
package metadata

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"latency-arbitrage-validator/internal/config"
)

// BuildSymbolMaps 构建 Symbol 映射表
// 从三家交易所获取元数据，并将用户输入的交易对映射到各交易所的具体标识符
// 参数 ctx: 上下文
// 参数 cfg: 配置
// 参数 f: 元数据获取器
// 返回: Symbol 映射表（key 为 Canon）
func BuildSymbolMaps(ctx context.Context, cfg *config.Config, f Fetcher) (map[string]*SymbolMap, error) {
	// 获取三家交易所的元数据
	okxInsts, err := f.FetchOKX(ctx, cfg.Metadata.OKX)
	if err != nil {
		return nil, fmt.Errorf("获取 OKX 元数据失败: %w", err)
	}

	binanceSyms, err := f.FetchBinance(ctx, cfg.Metadata.Binance)
	if err != nil {
		return nil, fmt.Errorf("获取 Binance 元数据失败: %w", err)
	}

	bittapData, err := f.FetchBittap(ctx, cfg.Metadata.Bittap)
	if err != nil {
		return nil, fmt.Errorf("获取 Bittap 元数据失败: %w", err)
	}

	// 构建各交易所的索引
	okxIndex := buildOKXIndex(okxInsts)
	binanceIndex := buildBinanceIndex(binanceSyms)
	bittapIndex := buildBittapIndex(bittapData)

	// 为每个用户配置的交易对构建映射
	result := make(map[string]*SymbolMap)
	for _, sym := range cfg.Symbols {
		mapping, err := buildMapping(sym.Input, okxIndex, binanceIndex, bittapIndex)
		if err != nil {
			return nil, fmt.Errorf("映射交易对 '%s' 失败: %w", sym.Input, err)
		}
		result[mapping.Canon] = mapping
	}

	return result, nil
}

type bittapIndexItem struct {
	symbol string
	depths []string
}

// buildOKXIndex 构建 OKX 合约索引
// 只索引 USDT 正向永续合约
// key: 标准化的交易对（如 BTCUSDT）
func buildOKXIndex(insts []OKXInstrument) map[string]*OKXInstrument {
	index := make(map[string]*OKXInstrument)
	for i := range insts {
		inst := &insts[i]
		if inst.IsUSDTLinearSwap() {
			// 从 instId 提取标准化交易对
			// BTC-USDT-SWAP -> BTCUSDT
			canon := normalizeSymbol(inst.Uly)
			index[canon] = inst
		}
	}
	return index
}

// buildBinanceIndex 构建 Binance 合约索引
// 只索引 USDT 永续合约
// key: 标准化的交易对（如 BTCUSDT）
func buildBinanceIndex(syms []BinanceSymbol) map[string]*BinanceSymbol {
	index := make(map[string]*BinanceSymbol)
	for i := range syms {
		sym := &syms[i]
		if sym.IsUSDTPerpetual() {
			// Binance symbol 已经是标准格式（如 BTCUSDT）
			canon := strings.ToUpper(sym.Symbol)
			index[canon] = sym
		}
	}
	return index
}

// buildBittapIndex 构建 Bittap 合约索引
// 索引现货和合约交易对
// key: 标准化的交易对（如 BTCUSDT）
func buildBittapIndex(data *BittapData) map[string]*bittapIndexItem {
	index := make(map[string]*bittapIndexItem)

	// 优先使用合约交易对（如果存在），否则回退到现货交易对。
	// 说明：验证阶段只需要可订阅的公共深度数据，不涉及任何真实交易或私有通道。
	if len(data.ContractSymbols) > 0 {
		for i := range data.ContractSymbols {
			sym := &data.ContractSymbols[i]
			if sym.QuoteCode != "USDT" {
				continue
			}
			if sym.Status != "" && sym.Status != "OPEN" && sym.Status != "TRADING" {
				continue
			}
			canon := normalizeSymbol(sym.SymbolId)
			index[canon] = &bittapIndexItem{symbol: sym.SymbolId, depths: sym.Depths}
		}
		if len(index) > 0 {
			return index
		}
	}

	if len(data.FuturesSymbols) > 0 {
		for i := range data.FuturesSymbols {
			sym := &data.FuturesSymbols[i]
			if sym.QuoteCode != "USDT" {
				continue
			}
			if sym.Status != "" && sym.Status != "OPEN" && sym.Status != "TRADING" {
				continue
			}
			canon := normalizeSymbol(sym.Symbol)
			index[canon] = &bittapIndexItem{symbol: sym.Symbol, depths: sym.Depths}
		}
		if len(index) > 0 {
			return index
		}
	}

	for i := range data.SpotSymbols {
		sym := &data.SpotSymbols[i]
		if sym.Status != "OPEN" || sym.QuoteCode != "USDT" {
			continue
		}
		canon := normalizeSymbol(sym.SymbolId)
		index[canon] = &bittapIndexItem{symbol: sym.SymbolId, depths: sym.Depths}
	}

	return index
}

// buildMapping 为单个交易对构建映射
// 参数 userInput: 用户输入的交易对，如 BTC-USDT
// 返回: 完整的 SymbolMap
func buildMapping(userInput string, okxIndex map[string]*OKXInstrument, binanceIndex map[string]*BinanceSymbol, bittapIndex map[string]*bittapIndexItem) (*SymbolMap, error) {
	// 标准化用户输入
	canon := normalizeSymbol(userInput)

	// 查找 OKX 合约
	okxInst, ok := okxIndex[canon]
	if !ok {
		return nil, fmt.Errorf("OKX 未找到交易对: %s", canon)
	}

	// 查找 Binance 合约
	binanceSym, ok := binanceIndex[canon]
	if !ok {
		return nil, fmt.Errorf("Binance 未找到交易对: %s", canon)
	}

	// 查找 Bittap 合约
	bittapSym, ok := bittapIndex[canon]
	if !ok {
		return nil, fmt.Errorf("Bittap 未找到交易对: %s", canon)
	}

	// 解析 tick size
	tickSize, err := strconv.ParseFloat(okxInst.TickSz, 64)
	if err != nil {
		tickSize = 0.01 // 默认值
	}

	// 获取 Bittap 深度档位（使用第一个）
	bittapTick := "0.1"
	if len(bittapSym.depths) > 0 {
		bittapTick = bittapSym.depths[0]
	}

	return &SymbolMap{
		Canon:      canon,
		UserInput:  userInput,
		OKXInstId:  okxInst.InstId,
		BinanceSym: strings.ToLower(binanceSym.Symbol),
		BittapSym:  bittapSym.symbol,
		BittapTick: bittapTick,
		TickSize:   tickSize,
	}, nil
}

// normalizeSymbol 标准化交易对格式
// 移除分隔符，转为大写
// 例如: BTC-USDT -> BTCUSDT, btc_usdt -> BTCUSDT
func normalizeSymbol(s string) string {
	// 移除常见分隔符
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "/", "")
	// 移除合约后缀
	s = strings.TrimSuffix(s, "SWAP")
	s = strings.TrimSuffix(s, "M")
	// 转为大写
	return strings.ToUpper(s)
}

// NormalizeToCanon 将用户输入转换为 Canon 格式
// 公开函数，供外部使用
func NormalizeToCanon(userInput string) string {
	return normalizeSymbol(userInput)
}
