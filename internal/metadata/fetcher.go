// Package metadata 负责从交易所获取合约元数据并构建 symbol 映射。
package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Fetcher 元数据获取器接口
// 定义从各交易所获取合约元数据的方法
type Fetcher interface {
	// FetchOKX 获取 OKX 合约元数据
	FetchOKX(ctx context.Context, url string) ([]OKXInstrument, error)
	// FetchBinance 获取 Binance 合约元数据
	FetchBinance(ctx context.Context, url string) ([]BinanceSymbol, error)
	// FetchBittap 获取 Bittap 合约元数据
	FetchBittap(ctx context.Context, url string) (*BittapData, error)
}

// HTTPFetcher HTTP 元数据获取器
// 通过 HTTP 请求获取各交易所的合约元数据
type HTTPFetcher struct {
	// client HTTP 客户端
	client *http.Client
}

// NewHTTPFetcher 创建 HTTP 元数据获取器
// 参数 timeoutMs: HTTP 请求超时时间（毫秒）
func NewHTTPFetcher(timeoutMs int) *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: time.Duration(timeoutMs) * time.Millisecond,
		},
	}
}

// FetchOKX 获取 OKX 合约元数据
// 参数 ctx: 上下文，用于取消请求
// 参数 url: OKX 合约元数据 API 地址
// 返回: OKX 合约列表
func (f *HTTPFetcher) FetchOKX(ctx context.Context, url string) ([]OKXInstrument, error) {
	body, err := f.doRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("请求 OKX 元数据失败: %w", err)
	}

	var resp OKXResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 OKX 元数据失败: %w", err)
	}

	if resp.Code != "0" {
		return nil, fmt.Errorf("OKX API 返回错误码: %s", resp.Code)
	}

	return resp.Data, nil
}

// FetchBinance 获取 Binance 合约元数据
// 参数 ctx: 上下文，用于取消请求
// 参数 url: Binance 合约元数据 API 地址
// 返回: Binance 交易对列表
func (f *HTTPFetcher) FetchBinance(ctx context.Context, url string) ([]BinanceSymbol, error) {
	body, err := f.doRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("请求 Binance 元数据失败: %w", err)
	}

	var resp BinanceResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 Binance 元数据失败: %w", err)
	}

	return resp.Symbols, nil
}

// FetchBittap 获取 Bittap 合约元数据
// 参数 ctx: 上下文，用于取消请求
// 参数 url: Bittap 合约元数据 API 地址
// 返回: Bittap 数据
func (f *HTTPFetcher) FetchBittap(ctx context.Context, url string) (*BittapData, error) {
	body, err := f.doRequest(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("请求 Bittap 元数据失败: %w", err)
	}

	var resp BittapResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析 Bittap 元数据失败: %w", err)
	}

	if resp.Code != "0" || !resp.Success {
		return nil, fmt.Errorf("Bittap API 返回错误: code=%s, msg=%s", resp.Code, resp.Msg)
	}

	return &resp.Data, nil
}

// doRequest 执行 HTTP GET 请求
// 参数 ctx: 上下文
// 参数 url: 请求地址
// 返回: 响应体字节数组
func (f *HTTPFetcher) doRequest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("User-Agent", "latency-arbitrage-validator/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	return body, nil
}
