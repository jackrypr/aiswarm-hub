// Package lmsr implements the Logarithmic Market Scoring Rule (LMSR)
// Originally developed by Robin Hanson for prediction markets.
//
// LMSR provides:
// - Bounded loss for market maker (max loss = b * ln(n) where n = number of outcomes)
// - Always available liquidity
// - Price = probability interpretation
// - Well-defined cost function
//
// Reference: "Logarithmic Market Scoring Rules for Modular Combinatorial Information Aggregation"
// by Robin Hanson, 2003, George Mason University
package lmsr

import (
	"math"
)

// LMSR implements the Logarithmic Market Scoring Rule
type LMSR struct {
	// B is the liquidity parameter (also called the "market depth" or "subsidy")
	// Higher B = more stable prices, less slippage, but more potential loss for market maker
	// Lower B = more volatile prices, more slippage, but less potential loss
	// Typical values: 100-1000 depending on expected market volume
	B float64
}

// New creates a new LMSR market maker with the given liquidity parameter
func New(liquidity float64) *LMSR {
	if liquidity <= 0 {
		liquidity = 100 // Default liquidity
	}
	return &LMSR{B: liquidity}
}

// Cost calculates the cost function C(q) = b * ln(sum of exp(q_i / b))
// For binary markets: C(qYes, qNo) = b * ln(exp(qYes/b) + exp(qNo/b))
func (l *LMSR) Cost(qYes, qNo float64) float64 {
	// Use log-sum-exp trick for numerical stability
	maxQ := math.Max(qYes, qNo)
	return l.B*maxQ/l.B + l.B*math.Log(math.Exp((qYes-maxQ)/l.B)+math.Exp((qNo-maxQ)/l.B))
}

// PriceYes returns the instantaneous price (probability) of the YES outcome
// Price = dC/dq_yes = exp(q_yes/b) / sum(exp(q_i/b))
func (l *LMSR) PriceYes(qYes, qNo float64) float64 {
	// Softmax function for numerical stability
	maxQ := math.Max(qYes, qNo)
	expYes := math.Exp((qYes - maxQ) / l.B)
	expNo := math.Exp((qNo - maxQ) / l.B)
	return expYes / (expYes + expNo)
}

// PriceNo returns the instantaneous price (probability) of the NO outcome
func (l *LMSR) PriceNo(qYes, qNo float64) float64 {
	return 1.0 - l.PriceYes(qYes, qNo)
}

// CostToBuy calculates the cost to buy `shares` of `outcome`
// Cost = C(q_new) - C(q_current)
func (l *LMSR) CostToBuy(qYes, qNo, shares float64, outcome string) float64 {
	currentCost := l.Cost(qYes, qNo)
	var newCost float64

	if outcome == "yes" || outcome == "YES" {
		newCost = l.Cost(qYes+shares, qNo)
	} else {
		newCost = l.Cost(qYes, qNo+shares)
	}

	return newCost - currentCost
}

// CostToSell calculates the proceeds from selling `shares` of `outcome`
// This is just the negative of buying (you get money back)
func (l *LMSR) CostToSell(qYes, qNo, shares float64, outcome string) float64 {
	return -l.CostToBuy(qYes, qNo, -shares, outcome)
}

// SharesForCost calculates how many shares you can buy for a given cost
// Uses binary search to find the answer
func (l *LMSR) SharesForCost(qYes, qNo, cost float64, outcome string) float64 {
	if cost <= 0 {
		return 0
	}

	// Binary search for the number of shares
	low := 0.0
	high := cost * 10 // Upper bound estimate

	for i := 0; i < 100; i++ { // Max iterations
		mid := (low + high) / 2
		midCost := l.CostToBuy(qYes, qNo, mid, outcome)

		if math.Abs(midCost-cost) < 0.0001 {
			return mid
		}

		if midCost < cost {
			low = mid
		} else {
			high = mid
		}
	}

	return (low + high) / 2
}

// NewProbabilityAfterBet calculates what the new YES probability would be
// after a bet of `amount` currency on `outcome`
func (l *LMSR) NewProbabilityAfterBet(qYes, qNo, amount float64, outcome string) float64 {
	shares := l.SharesForCost(qYes, qNo, amount, outcome)

	if outcome == "yes" || outcome == "YES" {
		return l.PriceYes(qYes+shares, qNo)
	}
	return l.PriceYes(qYes, qNo+shares)
}

// MaxLoss returns the maximum possible loss for the market maker
// For binary markets: b * ln(2)
func (l *LMSR) MaxLoss() float64 {
	return l.B * math.Log(2)
}

// MarketState represents the current state of an LMSR market
type MarketState struct {
	QYes        float64 `json:"qYes"`        // Outstanding YES shares
	QNo         float64 `json:"qNo"`         // Outstanding NO shares
	PriceYes    float64 `json:"priceYes"`    // Current YES probability/price
	PriceNo     float64 `json:"priceNo"`     // Current NO probability/price
	TotalVolume float64 `json:"totalVolume"` // Total trading volume
}

// GetMarketState returns the current state of the market
func (l *LMSR) GetMarketState(qYes, qNo, totalVolume float64) MarketState {
	return MarketState{
		QYes:        qYes,
		QNo:         qNo,
		PriceYes:    l.PriceYes(qYes, qNo),
		PriceNo:     l.PriceNo(qYes, qNo),
		TotalVolume: totalVolume,
	}
}

// SimulateBet shows what would happen if a bet is placed
type BetSimulation struct {
	SharesReceived  float64 `json:"sharesReceived"`
	Cost            float64 `json:"cost"`
	NewPriceYes     float64 `json:"newPriceYes"`
	NewPriceNo      float64 `json:"newPriceNo"`
	PriceImpact     float64 `json:"priceImpact"` // Change in price from this bet
	AveragePrice    float64 `json:"averagePrice"`
	PotentialPayout float64 `json:"potentialPayout"` // If outcome is correct
}

// SimulateBet shows the effect of placing a bet
func (l *LMSR) SimulateBet(qYes, qNo, amount float64, outcome string) BetSimulation {
	currentPriceYes := l.PriceYes(qYes, qNo)
	cost := amount
	shares := l.SharesForCost(qYes, qNo, amount, outcome)

	var newQYes, newQNo float64
	if outcome == "yes" || outcome == "YES" {
		newQYes = qYes + shares
		newQNo = qNo
	} else {
		newQYes = qYes
		newQNo = qNo + shares
	}

	newPriceYes := l.PriceYes(newQYes, newQNo)

	return BetSimulation{
		SharesReceived:  shares,
		Cost:            cost,
		NewPriceYes:     newPriceYes,
		NewPriceNo:      1 - newPriceYes,
		PriceImpact:     newPriceYes - currentPriceYes,
		AveragePrice:    cost / shares,
		PotentialPayout: shares, // Each share pays 1 unit if correct
	}
}
