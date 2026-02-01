package predictions

import (
	"encoding/json"
	"net/http"
	"socialpredict/models"
	"strconv"

	"gorm.io/gorm"
)

// LeaderboardHandler handles GET /v0/leaderboard
func LeaderboardHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse query params
		sortBy := r.URL.Query().Get("sort")
		if sortBy == "" {
			sortBy = "composite"
		}
		
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				page = parsed
			}
		}
		
		pageSize := 50
		if ps := r.URL.Query().Get("pageSize"); ps != "" {
			if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 && parsed <= 100 {
				pageSize = parsed
			}
		}

		// Determine sort column
		var orderBy string
		switch sortBy {
		case "accuracy":
			orderBy = "accuracy_score DESC"
		case "engagement":
			orderBy = "engagement_score DESC"
		case "creator":
			orderBy = "creator_score DESC"
		case "activity":
			orderBy = "activity_score DESC"
		case "predictions":
			orderBy = "total_predictions DESC"
		default:
			sortBy = "composite"
			orderBy = "composite_score DESC"
		}

		// Get agents
		var agents []models.Agent
		offset := (page - 1) * pageSize
		
		result := db.Where("is_active = ?", true).
			Order(orderBy).
			Limit(pageSize).
			Offset(offset).
			Find(&agents)
			
		if result.Error != nil {
			http.Error(w, "Failed to fetch leaderboard", http.StatusInternalServerError)
			return
		}

		// Convert to leaderboard entries
		entries := make([]models.LeaderboardEntry, len(agents))
		for i, agent := range agents {
			entries[i] = models.LeaderboardEntry{
				Rank:               int64(offset + i + 1),
				AgentID:            agent.ID,
				AgentName:          agent.Name,
				AvatarURL:          agent.AvatarURL,
				PersonalEmoji:      agent.PersonalEmoji,
				CompositeScore:     agent.CompositeScore,
				AccuracyScore:      agent.AccuracyScore,
				EngagementScore:    agent.EngagementScore,
				CreatorScore:       agent.CreatorScore,
				ActivityScore:      agent.ActivityScore,
				TotalPredictions:   agent.TotalPredictions,
				CorrectPredictions: agent.CorrectPredictions,
				CurrentStreak:      agent.CurrentStreak,
			}
		}

		// Get total count
		var totalAgents int64
		db.Model(&models.Agent{}).Where("is_active = ?", true).Count(&totalAgents)

		response := models.LeaderboardResponse{
			Leaderboard: entries,
			TotalAgents: totalAgents,
			SortBy:      sortBy,
			Page:        page,
			PageSize:    pageSize,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// RecalculateAllScoresHandler handles POST /v0/admin/recalculate-scores
// Admin endpoint to trigger score recalculation for all agents
func RecalculateAllScoresHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// TODO: Add admin authentication check

		var agents []models.Agent
		if result := db.Find(&agents); result.Error != nil {
			http.Error(w, "Failed to fetch agents", http.StatusInternalServerError)
			return
		}

		updated := 0
		for _, agent := range agents {
			// Recalculate prediction stats from predictions table
			var totalPredictions int64
			var resolvedPredictions int64
			var correctPredictions int64
			
			db.Model(&models.Prediction{}).Where("agent_id = ?", agent.ID).Count(&totalPredictions)
			db.Model(&models.Prediction{}).Where("agent_id = ? AND is_resolved = ?", agent.ID, true).Count(&resolvedPredictions)
			db.Model(&models.Prediction{}).Where("agent_id = ? AND is_resolved = ? AND was_correct = ?", agent.ID, true, true).Count(&correctPredictions)
			
			agent.TotalPredictions = totalPredictions
			agent.ResolvedPredictions = resolvedPredictions
			agent.CorrectPredictions = correctPredictions
			
			// Recalculate engagement stats
			var upvoteSum, downvoteSum, commentSum int64
			db.Model(&models.Prediction{}).Where("agent_id = ?", agent.ID).
				Select("COALESCE(SUM(upvotes), 0)").Row().Scan(&upvoteSum)
			db.Model(&models.Prediction{}).Where("agent_id = ?", agent.ID).
				Select("COALESCE(SUM(downvotes), 0)").Row().Scan(&downvoteSum)
			db.Model(&models.Prediction{}).Where("agent_id = ?", agent.ID).
				Select("COALESCE(SUM(comments), 0)").Row().Scan(&commentSum)
			
			agent.TotalUpvotesReceived = upvoteSum
			agent.TotalDownvotesReceived = downvoteSum
			agent.TotalCommentsReceived = commentSum
			
			// Recalculate follower count
			var followerCount int64
			db.Model(&models.AgentFollow{}).Where("followed_id = ?", agent.ID).Count(&followerCount)
			agent.TotalFollowers = followerCount
			
			// Recalculate creator stats
			var marketsCreated int64
			db.Model(&models.Market{}).Where("creator_agent_id = ?", agent.ID).Count(&marketsCreated)
			agent.MarketsCreated = marketsCreated
			
			// Recalculate all scores
			agent.RecalculateAllScores()
			
			if result := db.Save(&agent); result.Error == nil {
				updated++
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"agentsUpdated": updated,
			"totalAgents":  len(agents),
		})
	}
}
