package adminhandlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// DeleteMarketHandler handles DELETE /v0/admin/market/{id}
func DeleteMarketHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vars := mux.Vars(r)
		idStr := vars["id"]
		
		marketID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid market ID", http.StatusBadRequest)
			return
		}

		// Delete associated bets first
		db.Exec("DELETE FROM bets WHERE market_id = ?", marketID)
		db.Exec("DELETE FROM agent_bets WHERE market_id = ?", marketID)
		db.Exec("DELETE FROM predictions WHERE market_id = ?", marketID)
		
		// Delete the market
		result := db.Exec("DELETE FROM markets WHERE id = ?", marketID)
		if result.Error != nil {
			http.Error(w, "Failed to delete market", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"deleted": marketID,
		})
	}
}

// ResetOldStatsHandler handles POST /v0/admin/reset-old-stats
// Resets numUsers and old bet counts to 0
func ResetOldStatsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Reset numUsers-related counts (they're computed from old bets)
		// The markets table doesn't have numUsers directly but it's computed
		// from bets. We need to delete old agent_bets
		result := db.Exec("DELETE FROM agent_bets")
		if result.Error != nil {
			http.Error(w, "Failed to reset old bets", http.StatusInternalServerError)
			return
		}

		// Also delete old regular bets from agents
		db.Exec("DELETE FROM bets WHERE username LIKE 'agent:%'")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Old bet data cleared",
			"rowsAffected": result.RowsAffected,
		})
	}
}

// DeleteAgentHandler handles DELETE /v0/admin/agent/{id}
func DeleteAgentHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vars := mux.Vars(r)
		idStr := vars["id"]
		
		agentID, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid agent ID", http.StatusBadRequest)
			return
		}

		// Delete associated data first
		db.Exec("DELETE FROM agent_bets WHERE agent_id = ?", agentID)
		db.Exec("DELETE FROM predictions WHERE agent_id = ?", agentID)
		db.Exec("DELETE FROM agent_follows WHERE follower_id = ? OR followed_id = ?", agentID, agentID)
		db.Exec("DELETE FROM prediction_votes WHERE voter_id = ? AND voter_type = 'agent'", agentID)
		
		// Delete the agent
		result := db.Exec("DELETE FROM agents WHERE id = ?", agentID)
		if result.Error != nil {
			http.Error(w, "Failed to delete agent", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"deleted": agentID,
		})
	}
}
