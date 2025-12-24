# Product Overview

Latency arbitrage validation system for cryptocurrency USDT perpetual contracts.

## Purpose
Measure and validate price propagation delays between leading exchanges (OKX, Binance) and a follower exchange (Bittap) to determine if arbitrage opportunities exist that can cover fees and slippage.

## Current Phase: Research/Validation
- No real orders placed
- Shadow/paper execution only
- Measuring lead-lag timing distributions (millisecond-level)
- Simulating PnL with order book data

## Core Concept
Leader-Follower model where:
- OKX and Binance act as price leaders (faster price discovery)
- Bittap acts as follower (slower to update)
- All trades executed only on Bittap (in paper mode)

## Key Metrics
- Lead-lag delay distribution (P50/P90/P99)
- Spread opportunity frequency
- Shadow trade win rate and PnL after fees
- Expected Value (EV) calculations
