// Package bittap 实现 Bittap 交易所消息解析。
// 字段映射: ExchTsUnixMs=0, lastUpdateId -> Seq
package bittap

import (
	"encoding/json"
	"fmt"
	"strings"

	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/metadata"
	"latency-arbitrage-validator/internal/util/fastparse"
	"latency-arbitrage-validator/internal/util/timeutil"
)

// Parser Bittap 消息解析器
type Parser struct {
	// symbolMaps Symbol 映射表（key 为 Canon）
	symbolMaps map[string]*metadata.SymbolMap
}

// NewParser 创建 Bittap 消息解析器
// 参数 symbolMaps: Symbol 映射表（key 为 Canon）
func NewParser(symbolMaps map[string]*metadata.SymbolMap) *Parser {
	return &Parser{symbolMaps: symbolMaps}
}

// Parse 解析 Bittap WebSocket 消息为 BookEvent
// 参数 data: 原始消息字节
// 返回: 可能包含 0 或 1 个 BookEvent（非深度消息返回空切片）
func (p *Parser) Parse(data []byte) ([]*model.BookEvent, error) {
	if IsPong(data) {
		return nil, nil
	}

	arrivedAt := timeutil.NowNano()

	var msg DepthMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		// 可能是订阅响应或其它消息，尝试识别 PONG / result 消息已在上方处理
		return nil, fmt.Errorf("解析 Bittap 消息失败: %w", err)
	}

	if msg.Event != "f_depth30" {
		return nil, nil
	}

	canon := p.findCanonBySymbol(msg.Symbol)
	if canon == "" {
		return nil, nil
	}

	var bestBidPx, bestBidQty, bestAskPx, bestAskQty float64
	levels := make([]model.Level, 0, 10)

	if len(msg.Bids) > 0 && len(msg.Bids[0]) >= 2 {
		bestBidPx, _ = fastparse.ParseFloat(msg.Bids[0][0])
		bestBidQty, _ = fastparse.ParseFloat(msg.Bids[0][1])

		for i, bid := range msg.Bids {
			if i >= 5 || len(bid) < 2 {
				break
			}
			px, _ := fastparse.ParseFloat(bid[0])
			qty, _ := fastparse.ParseFloat(bid[1])
			levels = append(levels, model.Level{Price: px, Qty: qty})
		}
	}

	if len(msg.Asks) > 0 && len(msg.Asks[0]) >= 2 {
		bestAskPx, _ = fastparse.ParseFloat(msg.Asks[0][0])
		bestAskQty, _ = fastparse.ParseFloat(msg.Asks[0][1])

		for i, ask := range msg.Asks {
			if i >= 5 || len(ask) < 2 {
				break
			}
			px, _ := fastparse.ParseFloat(ask[0])
			qty, _ := fastparse.ParseFloat(ask[1])
			levels = append(levels, model.Level{Price: px, Qty: qty})
		}
	}

	event := &model.BookEvent{
		Exchange:        model.ExchangeBittap,
		SymbolCanon:     canon,
		BestBidPx:       bestBidPx,
		BestBidQty:      bestBidQty,
		BestAskPx:       bestAskPx,
		BestAskQty:      bestAskQty,
		Levels:          levels,
		ArrivedAtUnixNs: arrivedAt,
		ExchTsUnixMs:    0,
		Seq:             msg.LastUpdateID,
	}

	return []*model.BookEvent{event}, nil
}

// findCanonBySymbol 根据 Bittap Symbol 查找 Canon
// 参数 symbol: 如 BTC-USDT 或 BTC-USDT-M
func (p *Parser) findCanonBySymbol(symbol string) string {
	if symbol == "" {
		return ""
	}

	for _, m := range p.symbolMaps {
		if strings.EqualFold(m.BittapSym, symbol) {
			return m.Canon
		}
	}
	return ""
}

// IsPong 判断是否为 PONG 响应
// 支持 {"result":"PONG"} 或 {"method":"PONG"} 两种可能形式。
func IsPong(data []byte) bool {
	var pong PongResponse
	if err := json.Unmarshal(data, &pong); err != nil {
		return false
	}
	if pong.Method == "PONG" {
		return true
	}
	if pong.Result != nil && *pong.Result == "PONG" {
		return true
	}
	return false
}
