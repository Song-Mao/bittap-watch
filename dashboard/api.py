#!/usr/bin/env python3
"""
延迟套利验证器 - 监控 API 服务
端口: 8088
"""

import json
import os
from pathlib import Path
from datetime import datetime
from flask import Flask, jsonify, send_from_directory
from flask_cors import CORS

app = Flask(__name__, static_folder='static')
CORS(app)

OUTPUT_DIR = os.environ.get('OUTPUT_DIR', '/opt/latency-validator/output')

def load_jsonl(filename):
    """加载 JSONL 文件"""
    filepath = Path(OUTPUT_DIR) / filename
    records = []
    if not filepath.exists():
        return records
    with open(filepath, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if line:
                try:
                    records.append(json.loads(line))
                except json.JSONDecodeError:
                    pass
    return records

@app.route('/')
def index():
    return send_from_directory('static', 'index.html')

@app.route('/api/status')
def status():
    """系统状态概览"""
    metrics = load_jsonl('metrics.jsonl')
    if not metrics:
        return jsonify({'error': 'No metrics data'}), 404
    
    latest = metrics[-1]
    return jsonify({
        'timestamp': latest.get('ts_unix_ns', 0) / 1e9,
        'connections': {
            'okx': latest.get('okx', {}),
            'binance': latest.get('binance', {}),
            'bittap': latest.get('bittap', {})
        },
        'latency': {
            'okx': latest.get('latency_okx', {}),
            'binance': latest.get('latency_binance', {})
        },
        'ev': {
            'okx': latest.get('ev_okx', {}),
            'binance': latest.get('ev_binance', {})
        },
        'updates_per_sec': latest.get('updates_per_sec', [])
    })

@app.route('/api/metrics')
def metrics():
    """获取所有 metrics 历史"""
    data = load_jsonl('metrics.jsonl')
    return jsonify(data[-100:])

@app.route('/api/signals')
def signals():
    """获取信号记录"""
    data = load_jsonl('signals.jsonl')
    return jsonify(data[-100:])

@app.route('/api/trades')
def trades():
    """获取影子成交记录"""
    data = load_jsonl('paper_trades.jsonl')
    return jsonify(data[-100:])

@app.route('/api/summary')
def summary():
    """汇总统计"""
    metrics = load_jsonl('metrics.jsonl')
    signals = load_jsonl('signals.jsonl')
    
    if not metrics:
        return jsonify({'error': 'No data'}), 404
    
    latest = metrics[-1]
    
    # 从 metrics 的 EV 统计中获取交易数据（更准确）
    ev_okx = latest.get('ev_okx', {})
    ev_binance = latest.get('ev_binance', {})
    
    # 使用 EV 统计中的数据
    okx_count = ev_okx.get('Count', 0)
    okx_wins = ev_okx.get('WinCount', 0)
    okx_losses = ev_okx.get('LossCount', 0)
    
    binance_count = ev_binance.get('Count', 0)
    binance_wins = ev_binance.get('WinCount', 0)
    binance_losses = ev_binance.get('LossCount', 0)
    
    total_trades = okx_count + binance_count
    wins = okx_wins + binance_wins
    losses = okx_losses + binance_losses
    
    # 计算总 PnL（基于 EV 统计的平均盈亏）
    okx_pnl = (ev_okx.get('AvgProfit', 0) * okx_wins) - (ev_okx.get('AvgLoss', 0) * okx_losses)
    binance_pnl = (ev_binance.get('AvgProfit', 0) * binance_wins) - (ev_binance.get('AvgLoss', 0) * binance_losses)
    total_pnl = okx_pnl + binance_pnl
    
    return jsonify({
        'uptime_seconds': len(metrics) * 10,
        'total_signals': len(signals),
        'total_trades': total_trades,
        'total_pnl_bps': round(total_pnl, 2),
        'wins': wins,
        'losses': losses,
        'win_rate': round(wins / (wins + losses) * 100, 1) if (wins + losses) > 0 else 0,
        'by_leader': {
            'okx': {
                'trades': okx_count,
                'pnl': round(okx_pnl, 2)
            },
            'binance': {
                'trades': binance_count,
                'pnl': round(binance_pnl, 2)
            }
        },
        'latency': {
            'okx': latest.get('latency_okx', {}),
            'binance': latest.get('latency_binance', {})
        },
        'ev': {
            'okx': ev_okx,
            'binance': ev_binance
        }
    })

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8088, debug=False)
