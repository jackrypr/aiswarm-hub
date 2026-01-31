package agents

import (
	"encoding/json"
	"math"
	"net/http"
	"socialpredict/middleware"
	"socialpredict/models"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// SwarmConsensus represents the aggregated prediction from all agents
type SwarmConsensus struct {
	MarketID             int64               `json:"marketId"`
	ConsensusProbability float64             `json:"consensusProbability"` // Weighted average
	TotalAgents          int                 `json:"totalAgents"`
	TotalBets            int                 `json:"totalBets"`
	TotalWagered         int64               `json:"totalWagered"`
	AverageConfidence    float64             `json:"averageConfidence"`
	AverageReputation    float64             `json:"averageReputation"`
	Breakdown            SwarmBreakdown      `json:"breakdown"`
	TopPredictors        []AgentPrediction   `json:"topPredictors"`
}

// SwarmBreakdown shows the split between YES and NO predictions
type SwarmBreakdown struct {
	YesCount       int     `json:"yesCount"`
	NoCount        int     `json:"noCount"`
	YesWeight      float64 `json:"yesWeight"`
	NoWeight       float64 `json:"noWeight"`
	YesAmount      int64   `json:"yesAmount"`
	NoAmount       int64   `json:"noAmount"`
}

// AgentPrediction is a single agent's prediction for display
type AgentPrediction struct {
	AgentName   string  `json:"agentName"`
	Outcome     string  `json:"outcome"`
	Amount      int64   `json:"amount"`
	Confidence  float64 `json:"confidence"`
	Reputation  float64 `json:"reputation"`
	Weight      float64 `json:"weight"`
	Reasoning   string  `json:"reasoning,omitempty"`
}

// GetSwarmConsensusHandler handles GET /v0/markets/{marketId}/swarm
func GetSwarmConsensusHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract market ID from URL path
		// Expected path: /v0/markets/{marketId}/swarm
		pathParts := strings.Split(r.URL.Path, "/")
		var marketIDStr string
		for i, part := range pathParts {
			if part == "markets" && i+1 < len(pathParts) {
				marketIDStr = pathParts[i+1]
				break
			}
		}

		marketID, err := strconv.ParseInt(marketIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid market ID", http.StatusBadRequest)
			return
		}

		// Check market exists
		var market models.Market
		if result := db.First(&market, marketID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Market not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Get all agent bets for this market
		var agentBets []AgentBet
		if result := db.Where("market_id = ?", marketID).Find(&agentBets); result.Error != nil {
			http.Error(w, "Failed to fetch agent bets", http.StatusInternalServerError)
			return
		}

		// Get all agents who bet
		agentIDs := make([]int64, len(agentBets))
		for i, bet := range agentBets {
			agentIDs[i] = bet.AgentID
		}

		var agents []models.Agent
		if len(agentIDs) > 0 {
			db.Where("id IN ?", agentIDs).Find(&agents)
		}

		// Create agent lookup map
		agentMap := make(map[int64]models.Agent)
		for _, agent := range agents {
			agentMap[agent.ID] = agent
		}

		// Calculate weighted consensus
		consensus := calculateSwarmConsensus(agentBets, agentMap)
		consensus.MarketID = marketID

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(consensus)
	}
}

// calculateSwarmConsensus computes the weighted average prediction
func calculateSwarmConsensus(bets []AgentBet, agents map[int64]models.Agent) SwarmConsensus {
	if len(bets) == 0 {
		return SwarmConsensus{
			ConsensusProbability: 0.5, // Default neutral
		}
	}

	var (
		weightedYesSum   float64
		weightedNoSum    float64
		totalWeight      float64
		totalConfidence  float64
		totalReputation  float64
		yesCount         int
		noCount          int
		yesAmount        int64
		noAmount         int64
		topPredictors    []AgentPrediction
	)

	// Unique agents
	uniqueAgents := make(map[int64]bool)

	for _, bet := range bets {
		agent, exists := agents[bet.AgentID]
		if !exists {
			continue
		}

		uniqueAgents[agent.ID] = true

		// Calculate weight: reputation * confidence * log(amount + 1)
		// This gives more weight to:
		// 1. High-reputation agents
		// 2. High-confidence predictions
		// 3. Larger bets (with diminishing returns)
		reputationWeight := agent.Reputation
		confidenceWeight := bet.Confidence
		amountWeight := math.Log(float64(bet.Amount) + 1) / math.Log(100) // Normalize to ~1 for 100 unit bets

		weight := reputationWeight * confidenceWeight * amountWeight

		if bet.Outcome == "yes" {
			weightedYesSum += weight
			yesCount++
			yesAmount += bet.Amount
		} else {
			weightedNoSum += weight
			noCount++
			noAmount += bet.Amount
		}

		totalWeight += weight
		totalConfidence += bet.Confidence
		totalReputation += agent.Reputation

		// Track top predictors
		topPredictors = append(topPredictors, AgentPrediction{
			AgentName:  agent.Name,
			Outcome:    bet.Outcome,
			Amount:     bet.Amount,
			Confidence: bet.Confidence,
			Reputation: agent.Reputation,
			Weight:     weight,
			Reasoning:  bet.Reasoning,
		})
	}

	// Calculate consensus probability
	var consensusProbability float64
	if totalWeight > 0 {
		consensusProbability = weightedYesSum / totalWeight
	} else {
		consensusProbability = 0.5
	}

	// Sort top predictors by weight (descending)
	// Simple bubble sort for small arrays
	for i := 0; i < len(topPredictors)-1; i++ {
		for j := 0; j < len(topPredictors)-i-1; j++ {
			if topPredictors[j].Weight < topPredictors[j+1].Weight {
				topPredictors[j], topPredictors[j+1] = topPredictors[j+1], topPredictors[j]
			}
		}
	}

	// Limit to top 10
	if len(topPredictors) > 10 {
		topPredictors = topPredictors[:10]
	}

	avgConfidence := 0.0
	avgReputation := 0.0
	if len(bets) > 0 {
		avgConfidence = totalConfidence / float64(len(bets))
		avgReputation = totalReputation / float64(len(bets))
	}

	return SwarmConsensus{
		ConsensusProbability: consensusProbability,
		TotalAgents:          len(uniqueAgents),
		TotalBets:            len(bets),
		TotalWagered:         yesAmount + noAmount,
		AverageConfidence:    avgConfidence,
		AverageReputation:    avgReputation,
		Breakdown: SwarmBreakdown{
			YesCount:  yesCount,
			NoCount:   noCount,
			YesWeight: weightedYesSum,
			NoWeight:  weightedNoSum,
			YesAmount: yesAmount,
			NoAmount:  noAmount,
		},
		TopPredictors: topPredictors,
	}
}

// GetAgentLeaderboardHandler handles GET /v0/agents/leaderboard
func GetAgentLeaderboardHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get limit from query param
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		var agents []models.Agent
		if result := db.Where("is_claimed = true AND total_predictions > 0").
			Order("reputation DESC, total_predictions DESC").
			Limit(limit).
			Find(&agents); result.Error != nil {
			http.Error(w, "Failed to fetch leaderboard", http.StatusInternalServerError)
			return
		}

		// Convert to public format
		publicAgents := make([]models.AgentPublic, len(agents))
		for i, agent := range agents {
			publicAgents[i] = agent.ToPublic()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"agents":  publicAgents,
			"count":   len(publicAgents),
		})
	}
}

// GetAgentStatusHandler handles GET /v0/agents/status
func GetAgentStatusHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// This endpoint works with or without claiming
		agent, httpErr := middleware.ValidateAgentAPIKey(r, db)
		if httpErr != nil {
			// Return status indicating not authenticated
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "unauthenticated",
				"message": httpErr.Message,
			})
			return
		}

		status := "active"
		if !agent.IsClaimed {
			status = "pending_claim"
		} else if !agent.IsActive {
			status = "deactivated"
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  status,
			"agent":   agent.ToPublic(),
			"balance": agent.AccountBalance,
		})
	}
}
