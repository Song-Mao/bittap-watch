// Package store 维护所有交易所的最新订单簿状态。
// 使用单写者模式避免锁和竞态条件。
package store

import "latency-arbitrage-validator/internal/core/model"

// Store 最新订单簿缓存（单写者）
// 注意：本结构体默认由聚合器单 goroutine 写入；若要跨 goroutine 读，请通过消息或拷贝传递快照。
type Store struct {
	// books 按交易所、交易对缓存最新 BookEvent
	// 第一层 key: exchange（okx/binance/bittap）
	// 第二层 key: SymbolCanon（如 BTCUSDT）
	books map[string]map[string]*model.BookEvent
}

// New 创建新的订单簿缓存
func New() *Store {
	return &Store{
		books: make(map[string]map[string]*model.BookEvent, 3),
	}
}

// Update 更新缓存
// 参数 ev: 归一化后的订单簿事件
func (s *Store) Update(ev *model.BookEvent) {
	if ev == nil || ev.Exchange == "" || ev.SymbolCanon == "" {
		return
	}

	exBooks, ok := s.books[ev.Exchange]
	if !ok {
		exBooks = make(map[string]*model.BookEvent)
		s.books[ev.Exchange] = exBooks
	}
	exBooks[ev.SymbolCanon] = ev
}

// Get 获取指定交易所与交易对的最新订单簿
// 返回值可能为 nil；返回的指针应视为只读。
func (s *Store) Get(exchange, symbolCanon string) *model.BookEvent {
	exBooks, ok := s.books[exchange]
	if !ok {
		return nil
	}
	return exBooks[symbolCanon]
}

// GetPair 获取 Leader 与 Follower（Bittap）的订单簿快照
// 参数 leader: okx 或 binance
// 参数 symbolCanon: 统一交易对标识
func (s *Store) GetPair(leader, symbolCanon string) (leaderBook, followerBook *model.BookEvent) {
	leaderBook = s.Get(leader, symbolCanon)
	followerBook = s.Get(model.ExchangeBittap, symbolCanon)
	return leaderBook, followerBook
}
