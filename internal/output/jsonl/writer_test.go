// Package jsonl 输出模块测试
package jsonl

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"latency-arbitrage-validator/internal/core/model"
)

// **Feature: latency-arbitrage-validator, Property 19: Paper Trade Output Completeness**
// **Validates: Requirements 8.4**

func TestPaperTrade_OutputCompleteness_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("paper_trades JSON 必含必需字段", prop.ForAll(
		func(entryPx float64, exitPx float64, tEntry int64, tExit int64, leader string) bool {
			if leader == "" {
				leader = "okx"
			}
			pt := &model.PaperTrade{
				Leader:      leader,
				SymbolCanon: "BTCUSDT",
				Side:        "long",
				TEntryNs:    tEntry,
				TExitNs:     tExit,
				EntryPx:     entryPx,
				ExitPx:      exitPx,
				GrossPnLBps: 1,
				FeeBps:      1,
				NetPnLBps:   0,
				ExitReason:  "tp",
			}

			b, err := json.Marshal(pt)
			if err != nil {
				return false
			}

			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				return false
			}

			required := []string{
				"leader",
				"symbol_canon",
				"side",
				"t_entry_ns",
				"t_exit_ns",
				"entry_px",
				"exit_px",
				"gross_pnl_bps",
				"fee_bps",
				"net_pnl_bps",
				"exit_reason",
			}
			for _, k := range required {
				if _, ok := m[k]; !ok {
					return false
				}
			}
			return true
		},
		gen.Float64Range(1, 200000),
		gen.Float64Range(1, 200000),
		gen.Int64(),
		gen.Int64(),
		gen.OneConstOf("okx", "binance"),
	))

	properties.TestingRun(t)
}

func TestWriter_WriteAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")

	w, err := NewWriter(path, 100)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	for i := 0; i < 10; i++ {
		if err := w.Write(map[string]any{"i": i}); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lines := 0
	for sc.Scan() {
		lines++
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if lines != 10 {
		t.Fatalf("lines=%d, want 10", lines)
	}
}
