// Package main 是延迟套利验证器的入口点。
// 本验证器测量 OKX/Binance（领先交易所）与 Bittap（跟随交易所）之间
// USDT 永续合约的 lead-lag 时延，通过影子成交验证套利机会。
//
// 重要：本系统仅用于研究/验证，严禁真实下单。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	ossignal "os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"latency-arbitrage-validator/internal/config"
	"latency-arbitrage-validator/internal/core/model"
	"latency-arbitrage-validator/internal/core/paper"
	sigengine "latency-arbitrage-validator/internal/core/signal"
	"latency-arbitrage-validator/internal/core/store"
	"latency-arbitrage-validator/internal/exchange/binance"
	"latency-arbitrage-validator/internal/exchange/bittap"
	"latency-arbitrage-validator/internal/exchange/okx"
	"latency-arbitrage-validator/internal/metadata"
	"latency-arbitrage-validator/internal/output/jsonl"
	"latency-arbitrage-validator/internal/stats/ev"
	"latency-arbitrage-validator/internal/stats/latency"
	"latency-arbitrage-validator/internal/util/timeutil"
)

type rateKey struct {
	ex  string
	sym string
}

type metricsSnapshot struct {
	// TsUnixNs 指标采集时间（纳秒）
	TsUnixNs int64 `json:"ts_unix_ns"`

	// OKX OKX 连接指标
	OKX okx.ConnectionMetrics `json:"okx"`
	// Binance Binance 连接指标
	Binance binance.ConnectionMetrics `json:"binance"`
	// Bittap Bittap 连接指标
	Bittap bittap.ConnectionMetrics `json:"bittap"`

	// LatencyOKX OKX↙Bittap 时延统计
	LatencyOKX latency.LatencyStats `json:"latency_okx"`
	// LatencyBinance Binance↙Bittap 时延统计
	LatencyBinance latency.LatencyStats `json:"latency_binance"`

	// EVOKX OKX 链路 EV 统计
	EVOKX ev.EVStats `json:"ev_okx"`
	// EVBinance Binance 链路 EV 统计
	EVBinance ev.EVStats `json:"ev_binance"`

	// UpdatesPerSec 按交易所/交易对的更新速率（基于聚合器统计）
	UpdatesPerSec []updateRate `json:"updates_per_sec,omitempty"`
}

type updateRate struct {
	// Exchange 交易所
	Exchange string `json:"exchange"`
	// SymbolCanon 统一交易对
	SymbolCanon string `json:"symbol_canon"`
	// UpdatesPerSec 每秒更新次数
	UpdatesPerSec float64 `json:"updates_per_sec"`
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "配置验证失败: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.App.LogLevel)
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 捕获 SIGINT/SIGTERM，触发优雅退出
	sigCh := make(chan os.Signal, 2)
	ossignal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("收到退出信号，开始优雅关闭")
		cancel()
	}()

	// 启动时获取元数据并构建 symbol 映射（禁止硬编码订阅 symbol）
	fetcher := metadata.NewHTTPFetcher(cfg.Metadata.TimeoutMs)
	symbolMaps, err := metadata.BuildSymbolMaps(ctx, cfg, fetcher)
	if err != nil {
		logger.Error("构建 symbol 映射失败", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("symbol 映射完成", zap.Int("symbols", len(symbolMaps)))

	okxClient := okx.NewClient(&cfg.WS.OKX, symbolMaps, logger)
	binanceClient := binance.NewClient(&cfg.WS.Binance, symbolMaps, logger)
	bittapClient := bittap.NewClient(&cfg.WS.Bittap, symbolMaps, logger)

	startCtx, startCancel := context.WithTimeout(ctx, 10*time.Second)
	defer startCancel()

	if err := okxClient.Connect(startCtx); err != nil {
		logger.Error("OKX 连接失败", zap.Error(err))
		os.Exit(1)
	}
	if err := okxClient.Subscribe(); err != nil {
		logger.Error("OKX 订阅失败", zap.Error(err))
		os.Exit(1)
	}

	if err := binanceClient.Connect(startCtx); err != nil {
		logger.Error("Binance 连接失败", zap.Error(err))
		os.Exit(1)
	}
	if err := binanceClient.Subscribe(); err != nil {
		logger.Error("Binance 订阅失败", zap.Error(err))
		os.Exit(1)
	}

	if err := bittapClient.Connect(startCtx); err != nil {
		logger.Error("Bittap 连接失败", zap.Error(err))
		os.Exit(1)
	}
	if err := bittapClient.Subscribe(); err != nil {
		logger.Error("Bittap 订阅失败", zap.Error(err))
		os.Exit(1)
	}

	go okxClient.Run(ctx)
	go binanceClient.Run(ctx)
	go bittapClient.Run(ctx)

	var signalsWriter *jsonl.Writer
	var paperWriter *jsonl.Writer
	var metricsWriter *jsonl.Writer
	if cfg.Output.SignalsEnabled {
		signalsWriter, err = jsonl.NewWriter(fmt.Sprintf("%s/signals.jsonl", cfg.Output.Dir), cfg.Output.BufferSize)
		if err != nil {
			logger.Error("创建 signals writer 失败", zap.Error(err))
			os.Exit(1)
		}
	}
	if cfg.Output.PaperTradesEnabled {
		paperWriter, err = jsonl.NewWriter(fmt.Sprintf("%s/paper_trades.jsonl", cfg.Output.Dir), cfg.Output.BufferSize)
		if err != nil {
			logger.Error("创建 paper_trades writer 失败", zap.Error(err))
			os.Exit(1)
		}
	}
	if cfg.Output.MetricsEnabled {
		metricsWriter, err = jsonl.NewWriter(fmt.Sprintf("%s/metrics.jsonl", cfg.Output.Dir), cfg.Output.BufferSize)
		if err != nil {
			logger.Error("创建 metrics writer 失败", zap.Error(err))
			os.Exit(1)
		}
	}

	// 初始化核心组件（两条 Leader 链路独立）
	bookStore := store.New()
	latTracker := latency.NewTracker(10000)

	okxEngine := sigengine.NewEngine(model.ExchangeOKX, cfg.Strategy)
	binanceEngine := sigengine.NewEngine(model.ExchangeBinance, cfg.Strategy)
	okxExec := paper.NewExecutor(model.ExchangeOKX, cfg.Paper, cfg.Fees.Bittap)
	binanceExec := paper.NewExecutor(model.ExchangeBinance, cfg.Paper, cfg.Fees.Bittap)
	okxEV := ev.NewCalculator(1000)
	binanceEV := ev.NewCalculator(1000)

	if err := runAggregator(ctx, logger, bookStore, latTracker, okxEngine, binanceEngine, okxExec, binanceExec, okxEV, binanceEV, okxClient, binanceClient, bittapClient, signalsWriter, paperWriter, metricsWriter, cfg.Output.MetricsIntervalMs); err != nil {
		logger.Error("聚合器退出", zap.Error(err))
	}

	// 输出最后一条 metrics 快照（便于离线复盘）
	if metricsWriter != nil {
		nowNs := timeutil.NowNano()
		_ = metricsWriter.Write(metricsSnapshot{
			TsUnixNs:       nowNs,
			OKX:            okxClient.Metrics(),
			Binance:        binanceClient.Metrics(),
			Bittap:         bittapClient.Metrics(),
			LatencyOKX:     latTracker.Stats(model.ExchangeOKX),
			LatencyBinance: latTracker.Stats(model.ExchangeBinance),
			EVOKX:          okxEV.Stats(),
			EVBinance:      binanceEV.Stats(),
		})
		_ = metricsWriter.Flush()
	}

	// 优雅关闭（10s 超时）
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = okxClient.Close()
		_ = binanceClient.Close()
		_ = bittapClient.Close()
		if signalsWriter != nil {
			_ = signalsWriter.Close()
		}
		if paperWriter != nil {
			_ = paperWriter.Close()
		}
		if metricsWriter != nil {
			_ = metricsWriter.Close()
		}
	}()

	select {
	case <-shutdownCtx.Done():
		logger.Warn("关闭超时，强制退出")
	case <-done:
		logger.Info("关闭完成")
	}
}

func newLogger(level string) *zap.Logger {
	lvl := zapcore.InfoLevel
	if err := lvl.Set(level); err != nil {
		lvl = zapcore.InfoLevel
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build()
	if err != nil {
		return zap.NewNop()
	}
	return logger
}

func runAggregator(
	ctx context.Context,
	logger *zap.Logger,
	bookStore *store.Store,
	latTracker *latency.Tracker,
	okxEngine *sigengine.Engine,
	binanceEngine *sigengine.Engine,
	okxExec *paper.Executor,
	binanceExec *paper.Executor,
	okxEV *ev.Calculator,
	binanceEV *ev.Calculator,
	okxClient *okx.Client,
	binanceClient *binance.Client,
	bittapClient *bittap.Client,
	signalsWriter *jsonl.Writer,
	paperWriter *jsonl.Writer,
	metricsWriter *jsonl.Writer,
	metricsIntervalMs int,
) error {
	okxCh := okxClient.BookCh()
	binanceCh := binanceClient.BookCh()
	bittapCh := bittapClient.BookCh()

	if metricsIntervalMs <= 0 {
		metricsIntervalMs = 10000
	}
	metricsTicker := time.NewTicker(time.Duration(metricsIntervalMs) * time.Millisecond)
	defer metricsTicker.Stop()

	// 聚合器侧统计 updates_per_sec（按交易所/交易对）
	counts := make(map[rateKey]int64)
	lastCounts := make(map[rateKey]int64)
	lastMetricsAt := timeutil.NowNano()

	for {
		select {
		case <-ctx.Done():
			return nil

		case ev, ok := <-okxCh:
			if !ok {
				okxCh = nil
				continue
			}
			handleBookEvent(logger, bookStore, latTracker, okxEngine, binanceEngine, okxExec, binanceExec, okxEV, binanceEV, signalsWriter, paperWriter, ev, counts)

		case ev, ok := <-binanceCh:
			if !ok {
				binanceCh = nil
				continue
			}
			handleBookEvent(logger, bookStore, latTracker, okxEngine, binanceEngine, okxExec, binanceExec, okxEV, binanceEV, signalsWriter, paperWriter, ev, counts)

		case ev, ok := <-bittapCh:
			if !ok {
				bittapCh = nil
				continue
			}
			handleBookEvent(logger, bookStore, latTracker, okxEngine, binanceEngine, okxExec, binanceExec, okxEV, binanceEV, signalsWriter, paperWriter, ev, counts)

		case <-metricsTicker.C:
			if metricsWriter == nil {
				continue
			}

			nowNs := timeutil.NowNano()
			elapsedSec := float64(nowNs-lastMetricsAt) / 1e9
			if elapsedSec <= 0 {
				elapsedSec = float64(metricsIntervalMs) / 1000
			}

			var rates []updateRate
			for k, v := range counts {
				prev := lastCounts[k]
				qps := float64(v-prev) / elapsedSec
				rates = append(rates, updateRate{Exchange: k.ex, SymbolCanon: k.sym, UpdatesPerSec: qps})
				lastCounts[k] = v
			}
			lastMetricsAt = nowNs

			snap := metricsSnapshot{
				TsUnixNs:       nowNs,
				OKX:            okxClient.Metrics(),
				Binance:        binanceClient.Metrics(),
				Bittap:         bittapClient.Metrics(),
				LatencyOKX:     latTracker.Stats(model.ExchangeOKX),
				LatencyBinance: latTracker.Stats(model.ExchangeBinance),
				EVOKX:          okxEV.Stats(),
				EVBinance:      binanceEV.Stats(),
				UpdatesPerSec:  rates,
			}
			_ = metricsWriter.Write(snap)
			_ = metricsWriter.Flush()
		}

		if okxCh == nil && binanceCh == nil && bittapCh == nil {
			return nil
		}
	}
}

func handleBookEvent(
	logger *zap.Logger,
	bookStore *store.Store,
	latTracker *latency.Tracker,
	okxEngine *sigengine.Engine,
	binanceEngine *sigengine.Engine,
	okxExec *paper.Executor,
	binanceExec *paper.Executor,
	okxEV *ev.Calculator,
	binanceEV *ev.Calculator,
	signalsWriter *jsonl.Writer,
	paperWriter *jsonl.Writer,
	ev *model.BookEvent,
	counts map[rateKey]int64,
) {
	if ev == nil || ev.Exchange == "" || ev.SymbolCanon == "" {
		return
	}
	counts[rateKey{ex: ev.Exchange, sym: ev.SymbolCanon}]++

	bookStore.Update(ev)

	// 仅在 Follower 更新时记录时延（使用最新 Leader 快照）
	if ev.Exchange == model.ExchangeBittap {
		if okxBook, _ := bookStore.GetPair(model.ExchangeOKX, ev.SymbolCanon); okxBook != nil {
			latTracker.Add(okxBook, ev)
		}
		if binanceBook, _ := bookStore.GetPair(model.ExchangeBinance, ev.SymbolCanon); binanceBook != nil {
			latTracker.Add(binanceBook, ev)
		}
	}

	// 评估与执行（两条链路独立）
	okxBook, bittapBook := bookStore.GetPair(model.ExchangeOKX, ev.SymbolCanon)
	if okxBook != nil && bittapBook != nil {
		if sig := okxEngine.Evaluate(ev.ArrivedAtUnixNs, okxBook, bittapBook); sig != nil {
			applyEVAndMaybeOpen(sig, okxEV, okxExec, signalsWriter, paperWriter, logger)
		}
		if closed := okxExec.Evaluate(ev.ArrivedAtUnixNs, okxBook, bittapBook); closed != nil {
			okxEV.Add(closed)
			if closed.ExitReason == model.ExitSL {
				okxEngine.NotifyStopLoss(closed.SymbolCanon, ev.ArrivedAtUnixNs)
			}
			if paperWriter != nil {
				_ = paperWriter.Write(closed.ToPaperTrade(okxEV.Snapshot()))
			}
		}
	}

	binBook, bittapBook2 := bookStore.GetPair(model.ExchangeBinance, ev.SymbolCanon)
	if binBook != nil && bittapBook2 != nil {
		if sig := binanceEngine.Evaluate(ev.ArrivedAtUnixNs, binBook, bittapBook2); sig != nil {
			applyEVAndMaybeOpen(sig, binanceEV, binanceExec, signalsWriter, paperWriter, logger)
		}
		if closed := binanceExec.Evaluate(ev.ArrivedAtUnixNs, binBook, bittapBook2); closed != nil {
			binanceEV.Add(closed)
			if closed.ExitReason == model.ExitSL {
				binanceEngine.NotifyStopLoss(closed.SymbolCanon, ev.ArrivedAtUnixNs)
			}
			if paperWriter != nil {
				_ = paperWriter.Write(closed.ToPaperTrade(binanceEV.Snapshot()))
			}
		}
	}
}

func applyEVAndMaybeOpen(
	sig *model.Signal,
	evCalc *ev.Calculator,
	exec *paper.Executor,
	signalsWriter *jsonl.Writer,
	paperWriter *jsonl.Writer,
	logger *zap.Logger,
) {
	if sig == nil {
		return
	}

	// EV 拒绝：当 EV<0，标记信号但不执行影子成交
	evStats := evCalc.Stats()
	ev.ApplyRejection(sig, evStats)

	if signalsWriter != nil {
		_ = signalsWriter.Write(sig)
	}

	if sig.RejectedByEV {
		return
	}

	if _, opened, err := exec.TryOpen(sig); err != nil {
		logger.Warn("TryOpen 失败", zap.Error(err), zap.String("leader", sig.Leader), zap.String("symbol", sig.SymbolCanon))
		return
	} else if opened {
		// 开仓成功后立即不输出 paper trade；平仓时输出
		return
	}

	_ = paperWriter // 避免未使用（后续可能扩展为开仓事件输出）
}
