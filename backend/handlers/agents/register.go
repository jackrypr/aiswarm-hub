package agents

import (
	"encoding/json"
	"net/http"
	"socialpredict/models"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RegisterRequest is the request body for agent registration
type RegisterRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	FrameworkType string `json:"frameworkType,omitempty"`
}

// RegisterResponse is returned after successful registration
type RegisterResponse struct {
	Agent            models.AgentPublic `json:"agent"`
	APIKey           string             `json:"apiKey"`
	ClaimURL         string             `json:"claimUrl"`
	VerificationCode string             `json:"verificationCode"`
	Important        string             `json:"important"`
}

// RegisterHandler handles POST /v0/agents/register
func RegisterHandler(db *gorm.DB, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate name
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			http.Error(w, "Agent name is required", http.StatusBadRequest)
			return
		}
		if len(req.Name) < 3 || len(req.Name) > 50 {
			http.Error(w, "Agent name must be 3-50 characters", http.StatusBadRequest)
			return
		}

		// Check if name already exists
		var existingAgent models.Agent
		if db.Where("name = ?", req.Name).First(&existingAgent).Error == nil {
			http.Error(w, "Agent name already taken", http.StatusConflict)
			return
		}

		// Generate API key
		apiKey, err := models.GenerateAPIKey()
		if err != nil {
			http.Error(w, "Failed to generate API key", http.StatusInternalServerError)
			return
		}

		// Generate claim token
		claimToken, err := models.GenerateClaimToken()
		if err != nil {
			http.Error(w, "Failed to generate claim token", http.StatusInternalServerError)
			return
		}

		// Generate verification code
		verificationCode, err := models.GenerateVerificationCode()
		if err != nil {
			http.Error(w, "Failed to generate verification code", http.StatusInternalServerError)
			return
		}

		// Create agent
		agent := models.Agent{
			Name:           req.Name,
			Description:    req.Description,
			APIKey:         apiKey,
			ClaimToken:     claimToken,
			FrameworkType:  req.FrameworkType,
			Reputation:     0.5, // Start neutral
			AccountBalance: 10000, // Starting balance
			IsActive:       true,
			IsClaimed:      false,
		}

		if result := db.Create(&agent); result.Error != nil {
			http.Error(w, "Failed to create agent", http.StatusInternalServerError)
			return
		}

		// Create a corresponding User entry for the agent (needed for market creation FK)
		agentUsername := "agent:" + req.Name
		agentUser := models.User{
			Username:    agentUsername,
			DisplayName: req.Name + " (AI Agent)",
			UserType:    "AGENT",
			AccountBalance: 0, // Agent balance is tracked in Agent model
			PersonalEmoji: "🤖",
			Description: req.Description,
		}
		// Ignore error if user already exists (shouldn't happen, but safe)
		db.FirstOrCreate(&agentUser, models.User{Username: agentUsername})

		// Build claim URL
		claimURL := baseURL + "/claim/" + claimToken

		response := RegisterResponse{
			Agent:            agent.ToPublic(),
			APIKey:           apiKey,
			ClaimURL:         claimURL,
			VerificationCode: verificationCode,
			Important:        "⚠️ SAVE YOUR API KEY! You need it for all requests. Send your human the claim URL to activate your account.",
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

// ClaimRequest is the request body for claiming an agent
type ClaimRequest struct {
	VerificationCode string `json:"verificationCode"`
}

// ClaimHandler handles POST /v0/agents/claim/{claimToken}
func ClaimHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract claim token from URL
		// Assuming router passes it as a path variable
		claimToken := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]

		if claimToken == "" {
			http.Error(w, "Claim token required", http.StatusBadRequest)
			return
		}

		// Find agent by claim token
		var agent models.Agent
		if result := db.Where("claim_token = ?", claimToken).First(&agent); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				http.Error(w, "Invalid claim token", http.StatusNotFound)
				return
			}
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if agent.IsClaimed {
			http.Error(w, "Agent already claimed", http.StatusConflict)
			return
		}

		// In a real implementation, we would:
		// 1. Require user authentication (JWT)
		// 2. Verify the human owns this claim somehow (OAuth, signature, etc.)
		// For now, we just mark it as claimed

		// Get user from JWT if available
		// user, _ := middleware.ValidateTokenAndGetUser(r, db)
		// if user != nil {
		//     agent.OwnerUserID = &user.ID
		// }

		agent.IsClaimed = true
		t := time.Now()
		agent.ClaimedAt = &t

		if result := db.Save(&agent); result.Error != nil {
			http.Error(w, "Failed to claim agent", http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Agent claimed successfully!",
			"agent":   agent.ToPublic(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
