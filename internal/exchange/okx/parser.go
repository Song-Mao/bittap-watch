// Package okx 实现 OKX 交易所消息解析。
// 字段映射: ts -> ExchTsUnixMs, seqId -> Seq
package okx

import (
	"encoding/json"
	"fmt"

	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/metadata"
	"latency-arbitrage-validator/internal/util/fastparse"
	"latency-arbitrage-validator/internal/util/timeutil"
)

// Parser OKX 消息解析器
type Parser struct {
	// symbolMaps Symbol 映射表，用于将 instId 转换为 Canon
	symbolMaps map[string]*metadata.SymbolMap
}

// NewParser 创建 OKX 消息解析器
// 参数 symbolMaps: Symbol 映射表
func NewParser(symbolMaps map[string]*metadata.SymbolMap) *Parser {
	return &Parser{
		symbolMaps: symbolMaps,
	}
}

// Parse 解析 OKX WebSocket 消息
// 参数 data: 原始消息字节
// 返回: BookEvent 列表（一条消息可能包含多个数据）
func (p *Parser) Parse(data []byte) ([]*model.BookEvent, error) {
	// 记录到达时间（纳秒）
	arrivedAt := timeutil.NowNano()

	// 尝试解析为 books5 消息
	var msg Books5Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("解析 OKX 消息失败: %w", err)
	}

	// 检查是否为 books5 数据
	if msg.Arg.Channel != "books5" || len(msg.Data) == 0 {
		return nil, nil // 非 books5 消息，忽略
	}

	// 解析每条数据
	events := make([]*model.BookEvent, 0, len(msg.Data))
	for _, d := range msg.Data {
		event, err := p.parseBooks5Data(&d, arrivedAt)
		if err != nil {
			return nil, fmt.Errorf("解析 books5 数据失败: %w", err)
		}
		if event != nil {
			events = append(events, event)
		}
	}

	return events, nil
}

// parseBooks5Data 解析单条 books5 数据
// 参数 d: books5 数据
// 参数 arrivedAt: 到达时间（纳秒）
// 返回: BookEvent
func (p *Parser) parseBooks5Data(d *Books5Data, arrivedAt int64) (*model.BookEvent, error) {
	// 查找 Symbol 映射
	canon := p.findCanon(d.InstId)
	if canon == "" {
		return nil, nil // 未配置的交易对，忽略
	}

	// 解析时间戳
	// OKX ts 字段为毫秒字符串
	exchTs, err := fastparse.ParseInt(d.Ts)
	if err != nil {
		exchTs = 0
	}

	// 解析买卖盘
	// OKX bids/asks 格式: [[价格, 数量, 废弃, 订单数], ...]
	var bestBidPx, bestBidQty, bestAskPx, bestAskQty float64
	var levels []model.Level

	// 解析买盘（bids）
	if len(d.Bids) > 0 {
		bestBidPx, _ = fastparse.ParseFloat(d.Bids[0][0])
		bestBidQty, _ = fastparse.ParseFloat(d.Bids[0][1])

		for i, bid := range d.Bids {
			if i >= 5 {
				break
			}
			px, _ := fastparse.ParseFloat(bid[0])
			qty, _ := fastparse.ParseFloat(bid[1])
			levels = append(levels, model.Level{Price: px, Qty: qty})
		}
	}

	// 解析卖盘（asks）
	if len(d.Asks) > 0 {
		bestAskPx, _ = fastparse.ParseFloat(d.Asks[0][0])
		bestAskQty, _ = fastparse.ParseFloat(d.Asks[0][1])

		for i, ask := range d.Asks {
			if i >= 5 {
				break
			}
			px, _ := fastparse.ParseFloat(ask[0])
			qty, _ := fastparse.ParseFloat(ask[1])
			levels = append(levels, model.Level{Price: px, Qty: qty})
		}
	}

	return &model.BookEvent{
		Exchange:        model.ExchangeOKX,
		SymbolCanon:     canon,
		BestBidPx:       bestBidPx,
		BestBidQty:      bestBidQty,
		BestAskPx:       bestAskPx,
		BestAskQty:      bestAskQty,
		Levels:          levels,
		ArrivedAtUnixNs: arrivedAt,
		ExchTsUnixMs:    exchTs,
		Seq:             d.SeqId,
	}, nil
}

// findCanon 根据 instId 查找 Canon
// 参数 instId: OKX 合约 ID，如 BTC-USDT-SWAP
// 返回: Canon，如 BTCUSDT；未找到返回空字符串
func (p *Parser) findCanon(instId string) string {
	for _, m := range p.symbolMaps {
		if m.OKXInstId == instId {
			return m.Canon
		}
	}
	return ""
}

// IsSubscribeResponse 判断是否为订阅响应
func IsSubscribeResponse(data []byte) bool {
	var resp SubscribeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return false
	}
	return resp.Event == "subscribe" || resp.Event == "error"
}

// IsPong 判断是否为 pong 响应
func IsPong(data []byte) bool {
	return string(data) == "pong"
}
