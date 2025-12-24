// Package fastparse 提供高性能的字符串解析函数。
// 避免在热路径使用 fmt.Sprintf，使用 strconv 进行转换。
// 主要用于解析交易所 WebSocket 消息中的价格和数量字段。
package fastparse

import (
	"strconv"
)

// ParseFloat 快速解析浮点数字符串
// 使用 strconv.ParseFloat 实现，避免 fmt 包的额外开销
// 参数 s: 待解析的字符串，如 "12345.67"
// 返回: 解析后的浮点数和可能的错误
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(s, 64)
}

// ParseInt 快速解析整数字符串
// 使用 strconv.ParseInt 实现，支持 64 位整数
// 参数 s: 待解析的字符串，如 "12345"
// 返回: 解析后的整数和可能的错误
func ParseInt(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// ParseUint 快速解析无符号整数字符串
// 用于解析序列号等非负整数
// 参数 s: 待解析的字符串
// 返回: 解析后的无符号整数和可能的错误
func ParseUint(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

// MustParseFloat 解析浮点数，失败时返回 0
// 用于已知格式正确的场景，简化错误处理
// 参数 s: 待解析的字符串
// 返回: 解析后的浮点数，失败返回 0
func MustParseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// MustParseInt 解析整数，失败时返回 0
// 用于已知格式正确的场景，简化错误处理
// 参数 s: 待解析的字符串
// 返回: 解析后的整数，失败返回 0
func MustParseInt(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// FormatFloat 格式化浮点数为字符串
// 使用 strconv.FormatFloat 实现，避免 fmt.Sprintf 开销
// 参数 f: 待格式化的浮点数
// 参数 prec: 小数位数，-1 表示最短表示
// 返回: 格式化后的字符串
func FormatFloat(f float64, prec int) string {
	return strconv.FormatFloat(f, 'f', prec, 64)
}

// FormatInt 格式化整数为字符串
// 使用 strconv.FormatInt 实现
// 参数 i: 待格式化的整数
// 返回: 格式化后的字符串
func FormatInt(i int64) string {
	return strconv.FormatInt(i, 10)
}
