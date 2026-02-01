package predictions

import (
	"encoding/json"
	"net/http"
	"socialpredict/middleware"
	"socialpredict/models"
	"strconv"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// FollowAgentHandler handles POST /v0/agent/{id}/follow
func FollowAgentHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get follower agent
		follower, httpErr := middleware.ValidateAgentAPIKey(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}

		vars := mux.Vars(r)
		idStr := vars["id"]
		
		followedID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent ID", http.StatusBadRequest)
			return
		}

		// Can't follow yourself
		if follower.ID == followedID {
			http.Error(w, "Cannot follow yourself", http.StatusBadRequest)
			return
		}

		// Check target agent exists
		var followed models.Agent
		if result := db.First(&followed, followedID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Agent not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		tx := db.Begin()

		// Check if already following
		var existingFollow models.AgentFollow
		if result := db.Where("follower_id = ? AND followed_id = ?", follower.ID, followedID).First(&existingFollow); result.Error == nil {
			// Already following - unfollow
			tx.Delete(&existingFollow)
			
			// Update counts
			followed.TotalFollowers--
			follower.TotalFollowing--
			
			tx.Save(&followed)
			tx.Save(follower)
			
			// Recalculate engagement score
			followed.RecalculateEngagementScore()
			followed.RecalculateCompositeScore()
			tx.Save(&followed)
			
			tx.Commit()

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":    true,
				"following":  false,
				"followers":  followed.TotalFollowers,
			})
			return
		}

		// Create new follow
		follow := models.AgentFollow{
			FollowerID: follower.ID,
			FollowedID: followedID,
		}
		
		if result := tx.Create(&follow); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Failed to follow agent", http.StatusInternalServerError)
			return
		}

		// Update counts
		followed.TotalFollowers++
		follower.TotalFollowing++
		
		tx.Save(&followed)
		tx.Save(follower)

		// Recalculate engagement score
		followed.RecalculateEngagementScore()
		followed.RecalculateCompositeScore()
		tx.Save(&followed)
		
		tx.Commit()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"following":  true,
			"followers":  followed.TotalFollowers,
		})
	}
}

// UnfollowAgentHandler handles DELETE /v0/agent/{id}/follow
func UnfollowAgentHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get follower agent
		follower, httpErr := middleware.ValidateAgentAPIKey(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}

		vars := mux.Vars(r)
		idStr := vars["id"]
		
		followedID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent ID", http.StatusBadRequest)
			return
		}

		tx := db.Begin()

		// Find and delete follow
		var existingFollow models.AgentFollow
		if result := db.Where("follower_id = ? AND followed_id = ?", follower.ID, followedID).First(&existingFollow); result.Error != nil {
			tx.Rollback()
			http.Error(w, "Not following this agent", http.StatusBadRequest)
			return
		}

		tx.Delete(&existingFollow)

		// Update counts
		var followed models.Agent
		if result := db.First(&followed, followedID); result.Error == nil {
			followed.TotalFollowers--
			follower.TotalFollowing--
			
			tx.Save(&followed)
			tx.Save(follower)

			// Recalculate engagement score
			followed.RecalculateEngagementScore()
			followed.RecalculateCompositeScore()
			tx.Save(&followed)
		}
		
		tx.Commit()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"following":  false,
		})
	}
}

// GetAgentFollowersHandler handles GET /v0/agent/{id}/followers
func GetAgentFollowersHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		
		agentID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent ID", http.StatusBadRequest)
			return
		}

		// Parse pagination
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		// Get followers
		var follows []models.AgentFollow
		db.Where("followed_id = ?", agentID).Limit(limit).Find(&follows)

		// Get follower details
		followerIDs := make([]int64, len(follows))
		for i, f := range follows {
			followerIDs[i] = f.FollowerID
		}

		var followers []models.Agent
		if len(followerIDs) > 0 {
			db.Where("id IN ?", followerIDs).Find(&followers)
		}

		// Convert to public
		publicFollowers := make([]models.AgentPublic, len(followers))
		for i, f := range followers {
			publicFollowers[i] = f.ToPublic()
		}

		// Get total count
		var total int64
		db.Model(&models.AgentFollow{}).Where("followed_id = ?", agentID).Count(&total)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"followers": publicFollowers,
			"total":     total,
		})
	}
}

// GetAgentFollowingHandler handles GET /v0/agent/{id}/following
func GetAgentFollowingHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		
		agentID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent ID", http.StatusBadRequest)
			return
		}

		// Parse pagination
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
				limit = parsed
			}
		}

		// Get following
		var follows []models.AgentFollow
		db.Where("follower_id = ?", agentID).Limit(limit).Find(&follows)

		// Get followed agent details
		followedIDs := make([]int64, len(follows))
		for i, f := range follows {
			followedIDs[i] = f.FollowedID
		}

		var following []models.Agent
		if len(followedIDs) > 0 {
			db.Where("id IN ?", followedIDs).Find(&following)
		}

		// Convert to public
		publicFollowing := make([]models.AgentPublic, len(following))
		for i, f := range following {
			publicFollowing[i] = f.ToPublic()
		}

		// Get total count
		var total int64
		db.Model(&models.AgentFollow{}).Where("follower_id = ?", agentID).Count(&total)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"following": publicFollowing,
			"total":     total,
		})
	}
}

// GetAgentStatsHandler handles GET /v0/agent/{id}/stats
func GetAgentStatsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		idStr := vars["id"]
		
		agentID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent ID", http.StatusBadRequest)
			return
		}

		var agent models.Agent
		if result := db.First(&agent, agentID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Agent not found", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"stats":   agent.ToStats(),
		})
	}
}
