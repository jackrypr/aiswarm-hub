package predictions

import (
	"encoding/json"
	"net/http"
	"socialpredict/middleware"
	"socialpredict/models"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// MakePredictionHandler handles POST /v0/predict
// This is the new knowledge-based prediction endpoint (replaces betting)
func MakePredictionHandler(db *gorm.DB) http.HandlerFunc {
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

		var req models.PredictionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate request
		if req.MarketID <= 0 {
			http.Error(w, "Market ID is required", http.StatusBadRequest)
			return
		}
		
		outcome := strings.ToUpper(req.Outcome)
		if outcome != "YES" && outcome != "NO" {
			http.Error(w, "Outcome must be 'YES' or 'NO'", http.StatusBadRequest)
			return
		}
		
		// Default confidence to 50 if not provided
		confidence := req.Confidence
		if confidence <= 0 || confidence > 100 {
			confidence = 50
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

		// Check if agent already predicted on this market (optional - could allow changing)
		var existingPrediction models.Prediction
		if result := db.Where("agent_id = ? AND market_id = ?", agent.ID, req.MarketID).First(&existingPrediction); result.Error == nil {
			// Agent already predicted - update instead
			existingPrediction.Outcome = outcome
			existingPrediction.Confidence = confidence
			existingPrediction.Reasoning = req.Reasoning
			
			if result := db.Save(&existingPrediction); result.Error != nil {
				http.Error(w, "Failed to update prediction", http.StatusInternalServerError)
				return
			}
			
			response := models.PredictionResponse{
				Success:    true,
				Prediction: existingPrediction.ToPublic(),
				Message:    "Prediction updated",
			}
			
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Create new prediction
		prediction := models.Prediction{
			AgentID:     agent.ID,
			MarketID:    req.MarketID,
			Outcome:     outcome,
			Confidence:  confidence,
			Reasoning:   req.Reasoning,
			PredictedAt: time.Now(),
		}

		tx := db.Begin()

		if result := tx.Create(&prediction); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to create prediction", http.StatusInternalServerError)
			return
		}

		// Update agent stats and activity
		agent.TotalPredictions++
		agent.UpdateActivity()
		agent.RecalculateActivityScore()
		agent.RecalculateCompositeScore()
		
		if result := tx.Save(agent); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to update agent stats", http.StatusInternalServerError)
			return
		}

		// Update market prediction count
		market.TotalPredictions++
		if result := tx.Save(&market); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to update market stats", http.StatusInternalServerError)
			return
		}

		tx.Commit()

		// Load related data for response
		prediction.Agent = agent
		prediction.Market = &market

		response := models.PredictionResponse{
			Success:    true,
			Prediction: prediction.ToPublic(),
			Message:    "Prediction created successfully",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

// GetPredictionHandler handles GET /v0/prediction/{id}
func GetPredictionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid prediction ID", http.StatusBadRequest)
			return
		}

		var prediction models.Prediction
		if result := db.Preload("Agent").Preload("Market").First(&prediction, id); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Prediction not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"prediction": prediction.ToPublic(),
		})
	}
}

// GetAgentPredictionsHandler handles GET /v0/agent/{id}/predictions
func GetAgentPredictionsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		
		agentID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent ID", http.StatusBadRequest)
			return
		}

		// Parse query params
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}
		
		offset := 0
		if o := r.URL.Query().Get("offset"); o != "" {
			if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
				offset = parsed
			}
		}

		var predictions []models.Prediction
		result := db.Preload("Market").
			Where("agent_id = ?", agentID).
			Order("predicted_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&predictions)
			
		if result.Error != nil {
			http.Error(w, "Failed to fetch predictions", http.StatusInternalServerError)
			return
		}

		// Convert to public
		publicPredictions := make([]models.PredictionPublic, len(predictions))
		for i, p := range predictions {
			publicPredictions[i] = p.ToPublic()
		}

		// Get total count
		var total int64
		db.Model(&models.Prediction{}).Where("agent_id = ?", agentID).Count(&total)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"predictions": publicPredictions,
			"total":       total,
			"limit":       limit,
			"offset":      offset,
		})
	}
}

// GetMarketPredictionsHandler handles GET /v0/market/{id}/predictions
func GetMarketPredictionsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		
		marketID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid market ID", http.StatusBadRequest)
			return
		}

		// Parse query params
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		var predictions []models.Prediction
		result := db.Preload("Agent").
			Where("market_id = ?", marketID).
			Order("upvotes DESC, predicted_at DESC").
			Limit(limit).
			Find(&predictions)
			
		if result.Error != nil {
			http.Error(w, "Failed to fetch predictions", http.StatusInternalServerError)
			return
		}

		// Convert to public
		publicPredictions := make([]models.PredictionPublic, len(predictions))
		for i, p := range predictions {
			publicPredictions[i] = p.ToPublic()
		}

		// Calculate consensus
		yesCount := 0
		noCount := 0
		totalConfidence := 0.0
		for _, p := range predictions {
			if p.Outcome == "YES" {
				yesCount++
			} else {
				noCount++
			}
			totalConfidence += p.Confidence
		}
		
		avgConfidence := 0.0
		if len(predictions) > 0 {
			avgConfidence = totalConfidence / float64(len(predictions))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"predictions": publicPredictions,
			"total":       len(predictions),
			"consensus": map[string]interface{}{
				"yesCount":      yesCount,
				"noCount":       noCount,
				"avgConfidence": avgConfidence,
			},
		})
	}
}

// VotePredictionHandler handles POST /v0/prediction/{id}/vote
func VotePredictionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vars := mux.Vars(r)
		idStr := vars["id"]
		
		predictionID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid prediction ID", http.StatusBadRequest)
			return
		}

		// Get voter (agent or user)
		agent, agentErr := middleware.ValidateAgentAPIKey(r, db)
		
		var voterID int64
		var voterType string
		
		if agentErr == nil && agent != nil {
			voterID = agent.ID
			voterType = "agent"
		} else {
			// Try to get user from session (if logged in)
			// For now, require agent authentication
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		var req models.VoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		voteType := strings.ToLower(req.VoteType)
		if voteType != "up" && voteType != "down" {
			http.Error(w, "Vote type must be 'up' or 'down'", http.StatusBadRequest)
			return
		}

		// Check prediction exists
		var prediction models.Prediction
		if result := db.First(&prediction, predictionID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Prediction not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		// Can't vote on your own prediction
		if voterType == "agent" && prediction.AgentID == voterID {
			http.Error(w, "Cannot vote on your own prediction", http.StatusBadRequest)
			return
		}

		tx := db.Begin()

		// Check for existing vote
		var existingVote models.PredictionVote
		if result := db.Where("prediction_id = ? AND voter_id = ? AND voter_type = ?", 
			predictionID, voterID, voterType).First(&existingVote); result.Error == nil {
			
			// Remove old vote
			if existingVote.VoteType == "up" {
				prediction.Upvotes--
			} else {
				prediction.Downvotes--
			}
			
			if existingVote.VoteType == voteType {
				// Same vote - remove it (toggle off)
				tx.Delete(&existingVote)
			} else {
				// Different vote - change it
				existingVote.VoteType = voteType
				if voteType == "up" {
					prediction.Upvotes++
				} else {
					prediction.Downvotes++
				}
				tx.Save(&existingVote)
			}
		} else {
			// New vote
			vote := models.PredictionVote{
				PredictionID: predictionID,
				VoterID:      voterID,
				VoterType:    voterType,
				VoteType:     voteType,
			}
			tx.Create(&vote)
			
			if voteType == "up" {
				prediction.Upvotes++
			} else {
				prediction.Downvotes++
			}
		}

		tx.Save(&prediction)

		// Update prediction author's engagement score
		var author models.Agent
		if result := tx.First(&author, prediction.AgentID); result.Error == nil {
			author.TotalUpvotesReceived = 0
			author.TotalDownvotesReceived = 0
			
			// Recalculate total votes
			var upvoteSum, downvoteSum int64
			tx.Model(&models.Prediction{}).Where("agent_id = ?", author.ID).
				Select("COALESCE(SUM(upvotes), 0) as upvote_sum, COALESCE(SUM(downvotes), 0) as downvote_sum").
				Row().Scan(&upvoteSum, &downvoteSum)
			
			author.TotalUpvotesReceived = upvoteSum
			author.TotalDownvotesReceived = downvoteSum
			author.RecalculateEngagementScore()
			author.RecalculateCompositeScore()
			tx.Save(&author)
		}

		tx.Commit()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"upvotes":   prediction.Upvotes,
			"downvotes": prediction.Downvotes,
		})
	}
}
