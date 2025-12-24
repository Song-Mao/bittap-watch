// Package binance 实现 Binance 交易所消息解析。
// 字段映射: E -> ExchTsUnixMs, Seq=0
package binance

import (
	"encoding/json"
	"fmt"
	"strings"

	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/metadata"
	"latency-arbitrage-validator/internal/util/fastparse"
	"latency-arbitrage-validator/internal/util/timeutil"
)

// Parser Binance 消息解析器
type Parser struct {
	// symbolMaps Symbol 映射表（key 为 Canon），用于过滤未配置交易对
	symbolMaps map[string]*metadata.SymbolMap
}

// NewParser 创建 Binance 消息解析器
// 参数 symbolMaps: Symbol 映射表（key 为 Canon）
func NewParser(symbolMaps map[string]*metadata.SymbolMap) *Parser {
	return &Parser{symbolMaps: symbolMaps}
}

// Parse 解析 Binance WebSocket 消息为 BookEvent
// 参数 data: 原始消息字节
// 返回: 可能包含 0 或 1 个 BookEvent（非深度消息返回空切片）
func (p *Parser) Parse(data []byte) ([]*model.BookEvent, error) {
	arrivedAt := timeutil.NowNano()

	var msg DepthUpdate
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("解析 Binance 消息失败: %w", err)
	}

	if msg.EventType != "depthUpdate" {
		return nil, nil
	}

	canon := strings.ToUpper(msg.Symbol)
	if canon == "" {
		return nil, nil
	}
	if _, ok := p.symbolMaps[canon]; !ok {
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
		Exchange:        model.ExchangeBinance,
		SymbolCanon:     canon,
		BestBidPx:       bestBidPx,
		BestBidQty:      bestBidQty,
		BestAskPx:       bestAskPx,
		BestAskQty:      bestAskQty,
		Levels:          levels,
		ArrivedAtUnixNs: arrivedAt,
		ExchTsUnixMs:    msg.EventTimeMs,
		Seq:             0,
	}

	return []*model.BookEvent{event}, nil
}
