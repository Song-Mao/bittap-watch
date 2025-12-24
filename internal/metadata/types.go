// Package metadata 负责从交易所获取合约元数据并构建 symbol 映射。
package metadata

// OKXResponse OKX 合约元数据 API 响应
// API: GET /api/v5/public/instruments?instType=SWAP
type OKXResponse struct {
	// Code 响应码，"0" 表示成功
	Code string `json:"code"`
	// Data 合约列表
	Data []OKXInstrument `json:"data"`
}

// OKXInstrument OKX 合约信息
// 字段映射来自 OKX API 响应
type OKXInstrument struct {
	// InstId 合约 ID，如 BTC-USDT-SWAP
	InstId string `json:"instId"`
	// InstType 合约类型: SWAP（永续）, FUTURES（交割）
	InstType string `json:"instType"`
	// InstFamily 合约族，如 BTC-USDT
	InstFamily string `json:"instFamily"`
	// Uly 标的指数，如 BTC-USDT
	Uly string `json:"uly"`
	// CtType 合约类型: linear（正向）, inverse（反向）
	CtType string `json:"ctType"`
	// CtVal 合约面值
	CtVal string `json:"ctVal"`
	// CtValCcy 合约面值计价币种
	CtValCcy string `json:"ctValCcy"`
	// SettleCcy 结算币种: USDT, BTC 等
	SettleCcy string `json:"settleCcy"`
	// TickSz 最小价格变动单位
	TickSz string `json:"tickSz"`
	// LotSz 最小交易数量
	LotSz string `json:"lotSz"`
	// MinSz 最小下单数量
	MinSz string `json:"minSz"`
	// State 合约状态: live, suspend, preopen
	State string `json:"state"`
	// Lever 最大杠杆倍数
	Lever string `json:"lever"`
}

// IsUSDTLinearSwap 判断是否为 USDT 正向永续合约
// 条件: instType=SWAP, ctType=linear, settleCcy=USDT
func (i *OKXInstrument) IsUSDTLinearSwap() bool {
	return i.InstType == "SWAP" && i.CtType == "linear" && i.SettleCcy == "USDT"
}

// BinanceResponse Binance 合约元数据 API 响应
// API: GET /fapi/v1/exchangeInfo
type BinanceResponse struct {
	// Timezone 服务器时区
	Timezone string `json:"timezone"`
	// ServerTime 服务器时间
	ServerTime int64 `json:"serverTime"`
	// Symbols 交易对列表
	Symbols []BinanceSymbol `json:"symbols"`
}

// BinanceSymbol Binance 合约信息
// 字段映射来自 Binance Futures API 响应
type BinanceSymbol struct {
	// Symbol 交易对，如 BTCUSDT
	Symbol string `json:"symbol"`
	// Pair 标的交易对
	Pair string `json:"pair"`
	// ContractType 合约类型: PERPETUAL（永续）, CURRENT_QUARTER（当季）
	ContractType string `json:"contractType"`
	// Status 交易对状态: TRADING, BREAK
	Status string `json:"status"`
	// BaseAsset 标的资产，如 BTC
	BaseAsset string `json:"baseAsset"`
	// QuoteAsset 报价资产，如 USDT
	QuoteAsset string `json:"quoteAsset"`
	// MarginAsset 保证金资产
	MarginAsset string `json:"marginAsset"`
	// PricePrecision 价格精度
	PricePrecision int `json:"pricePrecision"`
	// QuantityPrecision 数量精度
	QuantityPrecision int `json:"quantityPrecision"`
	// Filters 过滤器列表
	Filters []BinanceFilter `json:"filters"`
}

// BinanceFilter Binance 过滤器
type BinanceFilter struct {
	// FilterType 过滤器类型: PRICE_FILTER, LOT_SIZE 等
	FilterType string `json:"filterType"`
	// TickSize 价格步长（PRICE_FILTER）
	TickSize string `json:"tickSize,omitempty"`
	// StepSize 数量步长（LOT_SIZE）
	StepSize string `json:"stepSize,omitempty"`
	// MinPrice 最小价格
	MinPrice string `json:"minPrice,omitempty"`
	// MaxPrice 最大价格
	MaxPrice string `json:"maxPrice,omitempty"`
	// MinQty 最小数量
	MinQty string `json:"minQty,omitempty"`
	// MaxQty 最大数量
	MaxQty string `json:"maxQty,omitempty"`
}

// IsUSDTPerpetual 判断是否为 USDT 永续合约
// 条件: contractType=PERPETUAL, quoteAsset=USDT, status=TRADING
func (s *BinanceSymbol) IsUSDTPerpetual() bool {
	return s.ContractType == "PERPETUAL" && s.QuoteAsset == "USDT" && s.Status == "TRADING"
}

// GetTickSize 获取价格步长
func (s *BinanceSymbol) GetTickSize() string {
	for _, f := range s.Filters {
		if f.FilterType == "PRICE_FILTER" {
			return f.TickSize
		}
	}
	return ""
}

// BittapResponse Bittap 合约元数据 API 响应
// API: GET /api/v1/exchangeInfo
type BittapResponse struct {
	// Code 响应码，"0" 表示成功
	Code string `json:"code"`
	// Msg 响应消息
	Msg string `json:"msg"`
	// Success 是否成功
	Success bool `json:"success"`
	// Data 数据
	Data BittapData `json:"data"`
}

// BittapData Bittap 响应数据
type BittapData struct {
	// Coins 币种列表
	Coins []BittapCoin `json:"coins"`
	// SpotSymbols 现货交易对列表
	SpotSymbols []BittapSpotSymbol `json:"spotSymbols"`
	// ContractSymbols 合约交易对列表（Bittap 实际字段为 contractSymbols）
	ContractSymbols []BittapContractSymbol `json:"contractSymbols"`
	// FuturesSymbols 合约交易对列表（兼容旧字段，如果有）
	FuturesSymbols []BittapFuturesSymbol `json:"futuresSymbols"`
}

// BittapCoin Bittap 币种信息
type BittapCoin struct {
	// CoinSymbol 币种代码，如 ETH
	CoinSymbol string `json:"coinSymbol"`
	// ID 币种 ID
	ID string `json:"id"`
	// ShortName 币种名称
	ShortName string `json:"shortName"`
	// Status 状态
	Status int `json:"status"`
}

// BittapSpotSymbol Bittap 现货交易对信息
type BittapSpotSymbol struct {
	// SymbolId 交易对 ID，如 ETH-USDT
	SymbolId string `json:"symbolId"`
	// SymbolName 交易对名称，如 ETH/USDT
	SymbolName string `json:"symbolName"`
	// BaseCode 基础币种，如 ETH
	BaseCode string `json:"baseCode"`
	// QuoteCode 报价币种，如 USDT
	QuoteCode string `json:"quoteCode"`
	// Status 状态: OPEN, CLOSE
	Status string `json:"status"`
	// PricePrecision 价格精度
	PricePrecision int `json:"pricePrecision"`
	// QuantityPrecision 数量精度
	QuantityPrecision int `json:"quantityPrecision"`
	// PriceStep 价格步长
	PriceStep string `json:"priceStep"`
	// Depths 深度精度列表
	Depths []string `json:"depths"`
	// MakerFeeRate Maker 手续费率
	MakerFeeRate string `json:"makerFeeRate"`
	// TakerFeeRate Taker 手续费率
	TakerFeeRate string `json:"takerFeeRate"`
}

// BittapContractSymbol Bittap 合约交易对信息（contractSymbols）
type BittapContractSymbol struct {
	// SymbolId 交易对，如 BTC-USDT-M
	SymbolId string `json:"symbolId"`
	// SymbolName 交易对名称，如 BTCUSDT
	SymbolName string `json:"symbolName"`
	// BaseCode 基础币种
	BaseCode string `json:"baseCode"`
	// QuoteCode 报价币种
	QuoteCode string `json:"quoteCode"`
	// Status 状态: OPEN, CLOSE
	Status string `json:"status"`
	// PriceStep 价格步长
	PriceStep string `json:"priceStep"`
	// Depths 深度精度列表
	Depths []string `json:"depths"`
	// IndexSymbolId 指数标的交易对，如 BTC-USDT
	IndexSymbolId string `json:"indexSymbolId"`
}

// BittapFuturesSymbol Bittap 合约交易对信息（兼容旧字段 futuresSymbols）
type BittapFuturesSymbol struct {
	// Symbol 交易对，如 BTC-USDT-M
	Symbol string `json:"symbol"`
	// SymbolName 交易对名称
	SymbolName string `json:"symbolName"`
	// BaseCode 基础币种
	BaseCode string `json:"baseCode"`
	// QuoteCode 报价币种
	QuoteCode string `json:"quoteCode"`
	// Status 状态
	Status string `json:"status"`
	// TickSize 价格步长
	TickSize string `json:"tickSize"`
	// Depths 深度精度列表
	Depths []string `json:"depths"`
}

// SymbolMap 交易对映射表
// 用于将用户输入的交易对映射到各交易所的具体标识符
type SymbolMap struct {
	// Canon 内部统一标识，如 BTCUSDT
	Canon string
	// UserInput 用户输入，如 BTC-USDT
	UserInput string
	// OKXInstId OKX 合约 ID，如 BTC-USDT-SWAP
	OKXInstId string
	// BinanceSym Binance 交易对，如 BTCUSDT（小写用于订阅）
	BinanceSym string
	// BittapSym Bittap 交易对，如 BTC-USDT-M
	BittapSym string
	// BittapTick Bittap 深度档位，如 0.1
	BittapTick string
	// TickSize 价格步长
	TickSize float64
}
