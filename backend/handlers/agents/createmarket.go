package agents

import (
	"encoding/json"
	"fmt"
	"net/http"
	"socialpredict/middleware"
	"socialpredict/models"
	"socialpredict/security"
	"strings"
	"time"

	"gorm.io/gorm"
)

const maxQuestionTitleLength = 160

// AgentCreateMarketRequest is the request body for creating a market as an agent
type AgentCreateMarketRequest struct {
	QuestionTitle      string    `json:"questionTitle"`
	Description        string    `json:"description"`
	ResolutionDateTime time.Time `json:"resolutionDateTime"`
	YesLabel           string    `json:"yesLabel,omitempty"`
	NoLabel            string    `json:"noLabel,omitempty"`
}

// AgentCreateMarketResponse is returned after creating a market
type AgentCreateMarketResponse struct {
	Success bool          `json:"success"`
	Market  models.Market `json:"market"`
	Message string        `json:"message,omitempty"`
}

// CreateMarketHandler handles POST /v0/agents/create
func CreateMarketHandler(db *gorm.DB) http.HandlerFunc {
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

		var req AgentCreateMarketRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Validate question title
		if len(req.QuestionTitle) < 1 || len(req.QuestionTitle) > maxQuestionTitleLength {
			http.Error(w, fmt.Sprintf("Question title must be 1-%d characters", maxQuestionTitleLength), http.StatusBadRequest)
			return
		}

		// Validate description
		if len(req.Description) > 2000 {
			http.Error(w, "Description must be less than 2000 characters", http.StatusBadRequest)
			return
		}

		// Validate resolution time (must be at least 1 hour in future)
		minResolutionTime := time.Now().Add(time.Hour)
		if req.ResolutionDateTime.Before(minResolutionTime) {
			http.Error(w, "Resolution time must be at least 1 hour in the future", http.StatusBadRequest)
			return
		}

		// Initialize security service for sanitization
		securityService := security.NewSecurityService()
		marketInput := security.MarketInput{
			Title:       req.QuestionTitle,
			Description: req.Description,
			EndTime:     req.ResolutionDateTime.String(),
		}
		sanitizedInput, err := securityService.ValidateAndSanitizeMarketInput(marketInput)
		if err != nil {
			http.Error(w, "Invalid market data: "+err.Error(), http.StatusBadRequest)
			return
		}

		// Set default labels
		yesLabel := strings.TrimSpace(req.YesLabel)
		noLabel := strings.TrimSpace(req.NoLabel)
		if yesLabel == "" {
			yesLabel = "YES"
		}
		if noLabel == "" {
			noLabel = "NO"
		}

		// Validate labels
		if len(yesLabel) > 20 || len(noLabel) > 20 {
			http.Error(w, "Labels must be 20 characters or less", http.StatusBadRequest)
			return
		}

		// Get the corresponding user for this agent
		// Agent username is "agent:<name>"
		agentUsername := fmt.Sprintf("agent:%s", agent.Name)

		// Create agent user if not exists (needed for FK constraint)
		displayName := fmt.Sprintf("%s AI Agent", agent.Name)
		agentEmail := fmt.Sprintf("agent-%d-%d@binkaroni.local", agent.ID, time.Now().UnixNano())
		
		// First check if user already exists
		var existingUser models.User
		userExists := db.Where("username = ?", agentUsername).First(&existingUser).Error == nil
		
		if !userExists {
			// Insert user using raw SQL
			insertSQL := `INSERT INTO users (username, display_name, user_type, email, password, account_balance, personal_emoji, must_change_password, created_at, updated_at) 
				VALUES ($1, $2, 'AGENT', $3, 'AGENT_NO_LOGIN', 0, '🤖', false, NOW(), NOW())`
			
			if err := db.Exec(insertSQL, agentUsername, displayName, agentEmail).Error; err != nil {
				http.Error(w, fmt.Sprintf("Failed to create agent user: %s (username: %s, email: %s)", err.Error(), agentUsername, agentEmail), http.StatusInternalServerError)
				return
			}
			
			// Verify user was created
			if db.Where("username = ?", agentUsername).First(&existingUser).Error != nil {
				http.Error(w, fmt.Sprintf("User not found after insert - username: %s", agentUsername), http.StatusInternalServerError)
				return
			}
		}

		// Create the market
		newMarket := models.Market{
			QuestionTitle:      sanitizedInput.Title,
			Description:        sanitizedInput.Description,
			ResolutionDateTime: req.ResolutionDateTime,
			YesLabel:           yesLabel,
			NoLabel:            noLabel,
			CreatorUsername:    agentUsername,
		}

		marketResult := db.Create(&newMarket)
		if marketResult.Error != nil {
			// Add debug info
			debugMsg := fmt.Sprintf("Error creating market (creator: %s, agentName: %s, agentID: %d): %s", 
				agentUsername, agent.Name, agent.ID, marketResult.Error.Error())
			http.Error(w, debugMsg, http.StatusInternalServerError)
			return
		}

		// Return success response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(AgentCreateMarketResponse{
			Success: true,
			Market:  newMarket,
			Message: "Market created successfully",
		})
	}
}
