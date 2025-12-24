// Package jsonl 实现异步 JSONL 文件写入。
// 使用带缓冲的 channel 实现热路径的非阻塞写入。
package jsonl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

type opType int

const (
	opWrite opType = iota
	opFlush
	opClose
)

type op struct {
	typ  opType
	val  any
	done chan error
}

// Writer 异步 JSONL 写入器
// Write 只负责投递，实际 JSON 编码与文件 I/O 在后台 goroutine 完成。
type Writer struct {
	// path 输出文件路径
	path string
	// ch 操作通道
	ch chan op

	closeOnce sync.Once
	closeErr  error
	closed    int32

	sendMu sync.Mutex

	wg sync.WaitGroup
}

// NewWriter 创建 JSONL 写入器
// 参数 path: 输出文件路径
// 参数 bufferSize: 写入缓冲区大小（channel capacity）
func NewWriter(path string, bufferSize int) (*Writer, error) {
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("打开输出文件失败: %w", err)
	}

	w := &Writer{
		path: path,
		ch:   make(chan op, bufferSize),
	}

	w.wg.Add(1)
	go w.loop(f)

	return w, nil
}

// Write 异步写入一条 JSONL 记录
func (w *Writer) Write(v any) error {
	if w == nil {
		return fmt.Errorf("writer 为空")
	}
	if atomic.LoadInt32(&w.closed) == 1 {
		return fmt.Errorf("writer 已关闭")
	}
	w.sendMu.Lock()
	defer w.sendMu.Unlock()
	if atomic.LoadInt32(&w.closed) == 1 {
		return fmt.Errorf("writer 已关闭")
	}
	w.ch <- op{typ: opWrite, val: v}
	return nil
}

// Flush 强制 flush 文件缓冲区
func (w *Writer) Flush() error {
	if w == nil {
		return nil
	}
	if atomic.LoadInt32(&w.closed) == 1 {
		return nil
	}
	w.sendMu.Lock()
	defer w.sendMu.Unlock()
	if atomic.LoadInt32(&w.closed) == 1 {
		return nil
	}
	done := make(chan error, 1)
	w.ch <- op{typ: opFlush, done: done}
	return <-done
}

// Close 关闭写入器（会先 flush）
func (w *Writer) Close() error {
	if w == nil {
		return nil
	}
	w.closeOnce.Do(func() {
		atomic.StoreInt32(&w.closed, 1)
		w.sendMu.Lock()
		defer w.sendMu.Unlock()
		done := make(chan error, 1)
		w.ch <- op{typ: opClose, done: done}
		w.closeErr = <-done
		close(w.ch)
	})
	w.wg.Wait()
	return w.closeErr
}

func (w *Writer) loop(f *os.File) {
	defer w.wg.Done()
	defer f.Close()

	bw := bufio.NewWriterSize(f, 1<<20) // 1MB buffer
	encErr := func(err error, done chan error) {
		if done != nil {
			done <- err
		}
	}

	for req := range w.ch {
		switch req.typ {
		case opWrite:
			b, err := json.Marshal(req.val)
			if err != nil {
				continue
			}
			if _, err := bw.Write(b); err != nil {
				continue
			}
			if err := bw.WriteByte('\n'); err != nil {
				continue
			}
		case opFlush:
			encErr(bw.Flush(), req.done)
		case opClose:
			err := bw.Flush()
			encErr(err, req.done)
			return
		}
	}
}
