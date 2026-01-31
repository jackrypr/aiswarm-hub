package agents

import (
	"encoding/json"
	"net/http"
	"socialpredict/middleware"
	"socialpredict/models"
	"time"

	"gorm.io/gorm"
)

// AgentBet extends the base Bet model with agent-specific fields
type AgentBet struct {
	gorm.Model
	ID         int64   `json:"id" gorm:"primary_key"`
	AgentID    int64   `json:"agentId" gorm:"not null;index"`
	MarketID   int64   `json:"marketId" gorm:"not null;index"`
	Amount     int64   `json:"amount" gorm:"not null"`
	Outcome    string  `json:"outcome" gorm:"not null"` // "yes" or "no"
	Confidence float64 `json:"confidence" gorm:"default:0.5"` // 0-1, how confident the agent is
	Reasoning  string  `json:"reasoning,omitempty" gorm:"size:1000"`
	PlacedAt   time.Time `json:"placedAt"`
	
	// Calculated fields
	SharesReceived float64 `json:"sharesReceived"`
	AveragePrice   float64 `json:"averagePrice"`
}

// AgentBetRequest is the request body for placing an agent bet
type AgentBetRequest struct {
	MarketID   int64   `json:"marketId"`
	Amount     int64   `json:"amount"`
	Outcome    string  `json:"outcome"`    // "yes" or "no"
	Confidence float64 `json:"confidence"` // 0-1
	Reasoning  string  `json:"reasoning,omitempty"`
}

// AgentBetResponse is returned after placing a bet
type AgentBetResponse struct {
	Success        bool            `json:"success"`
	Bet            AgentBet        `json:"bet"`
	NewBalance     int64           `json:"newBalance"`
	MarketState    MarketStateInfo `json:"marketState"`
}

// MarketStateInfo provides current market state
type MarketStateInfo struct {
	PriceYes    float64 `json:"priceYes"`
	PriceNo     float64 `json:"priceNo"`
	TotalVolume int64   `json:"totalVolume"`
}

// PlaceBetHandler handles POST /v0/agents/bet
func PlaceBetHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Validate agent (must be claimed)
		agent, httpErr := middleware.ValidateClaimedAgent(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}

		var req AgentBetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.MarketID <= 0 {
			http.Error(w, "Market ID is required", http.StatusBadRequest)
			return
		}
		if req.Amount <= 0 {
			http.Error(w, "Amount must be positive", http.StatusBadRequest)
			return
		}
		if req.Outcome != "yes" && req.Outcome != "no" {
			http.Error(w, "Outcome must be 'yes' or 'no'", http.StatusBadRequest)
			return
		}
		if req.Confidence < 0 || req.Confidence > 1 {
			http.Error(w, "Confidence must be between 0 and 1", http.StatusBadRequest)
			return
		}

		// Check agent has sufficient balance
		if agent.AccountBalance < req.Amount {
			http.Error(w, "Insufficient balance", http.StatusBadRequest)
			return
		}

		// Check market exists and is active
		var market models.Market
		if result := db.First(&market, req.MarketID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Market not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if market.IsResolved {
			http.Error(w, "Market is already resolved", http.StatusBadRequest)
			return
		}

		// Start transaction
		tx := db.Begin()

		// Deduct from agent balance
		agent.AccountBalance -= req.Amount
		if result := tx.Save(agent); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to update balance", http.StatusInternalServerError)
			return
		}

		// Create the agent bet
		bet := AgentBet{
			AgentID:    agent.ID,
			MarketID:   req.MarketID,
			Amount:     req.Amount,
			Outcome:    req.Outcome,
			Confidence: req.Confidence,
			Reasoning:  req.Reasoning,
			PlacedAt:   time.Now(),
		}

		if result := tx.Create(&bet); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to place bet", http.StatusInternalServerError)
			return
		}

		// Also create a standard bet for compatibility with existing system
		standardBet := models.Bet{
			Username: "agent:" + agent.Name, // Prefix to distinguish from human users
			MarketID: uint(req.MarketID), // Convert int64 to uint for compatibility
			Amount:   req.Amount,
			Outcome:  req.Outcome,
			PlacedAt: time.Now(),
		}

		if result := tx.Create(&standardBet); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to create standard bet", http.StatusInternalServerError)
			return
		}

		// Update agent's prediction count
		agent.TotalPredictions++
		agent.TotalWagered += req.Amount
		if result := tx.Save(agent); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to update agent stats", http.StatusInternalServerError)
			return
		}

		tx.Commit()

		response := AgentBetResponse{
			Success:    true,
			Bet:        bet,
			NewBalance: agent.AccountBalance,
			MarketState: MarketStateInfo{
				// These would be calculated from the market's current state
				PriceYes: 0.5, // Placeholder - would use LMSR calculation
				PriceNo:  0.5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

// GetAgentBetsHandler handles GET /v0/agents/bets
func GetAgentBetsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Validate agent
		agent, httpErr := middleware.ValidateAgentAPIKey(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}

		var bets []AgentBet
		if result := db.Where("agent_id = ?", agent.ID).Order("placed_at DESC").Find(&bets); result.Error != nil {
			http.Error(w, "Failed to fetch bets", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"bets":    bets,
			"count":   len(bets),
		})
	}
}

// GetMarketAgentBetsHandler handles GET /v0/markets/{marketId}/agent-bets
func GetMarketAgentBetsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract market ID from URL (would be done by router)
		// marketID := extractMarketID(r)

		// For now, return all agent bets on this market
		var bets []AgentBet
		// db.Where("market_id = ?", marketID).Find(&bets)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"bets":    bets,
		})
	}
}
