package middleware

import (
	"net/http"
	"socialpredict/models"
	"strings"

	"gorm.io/gorm"
)

// HTTPError for agent auth errors
type AgentHTTPError struct {
	StatusCode int
	Message    string
}

// ValidateAgentAPIKey validates an agent's API key and returns the agent
func ValidateAgentAPIKey(r *http.Request, db *gorm.DB) (*models.Agent, *HTTPError) {
	// Try X-Agent-API-Key header first
	apiKey := r.Header.Get("X-Agent-API-Key")

	// Fallback to Authorization header with "Agent" prefix
	if apiKey == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Agent ") {
			apiKey = strings.TrimPrefix(authHeader, "Agent ")
		} else if strings.HasPrefix(authHeader, "Bearer swarm_sk_") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if apiKey == "" {
		return nil, &HTTPError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Agent API key required. Use X-Agent-API-Key header or 'Agent <key>' in Authorization header",
		}
	}

	// Validate API key format
	if !strings.HasPrefix(apiKey, "swarm_sk_") {
		return nil, &HTTPError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Invalid API key format",
		}
	}

	// Look up agent in database
	var agent models.Agent
	result := db.Where("api_key = ?", apiKey).First(&agent)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, &HTTPError{
				StatusCode: http.StatusUnauthorized,
				Message:    "Invalid agent API key",
			}
		}
		return nil, &HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Database error validating agent",
		}
	}

	// Check if agent is active
	if !agent.IsActive {
		return nil, &HTTPError{
			StatusCode: http.StatusForbidden,
			Message:    "Agent account is deactivated",
		}
	}

	// Check if agent is claimed (required for betting, optional for status checks)
	// This check can be enforced at the handler level if needed

	return &agent, nil
}

// ValidateClaimedAgent validates that an agent is both authenticated and claimed
func ValidateClaimedAgent(r *http.Request, db *gorm.DB) (*models.Agent, *HTTPError) {
	agent, httpErr := ValidateAgentAPIKey(r, db)
	if httpErr != nil {
		return nil, httpErr
	}

	if !agent.IsClaimed {
		return nil, &HTTPError{
			StatusCode: http.StatusForbidden,
			Message:    "Agent must be claimed by a human owner before participating in markets",
		}
	}

	return agent, nil
}

// ValidateAgentOrUser attempts to validate as agent first, then falls back to user
// Returns agent, user, and error - one of agent/user will be non-nil on success
func ValidateAgentOrUser(r *http.Request, db *gorm.DB) (*models.Agent, *models.User, *HTTPError) {
	// Check for agent API key first
	agentKey := r.Header.Get("X-Agent-API-Key")
	authHeader := r.Header.Get("Authorization")

	// If it looks like an agent request, validate as agent
	if agentKey != "" || strings.HasPrefix(authHeader, "Agent ") || strings.Contains(authHeader, "swarm_sk_") {
		agent, httpErr := ValidateAgentAPIKey(r, db)
		if httpErr != nil {
			return nil, nil, httpErr
		}
		return agent, nil, nil
	}

	// Otherwise validate as user
	user, httpErr := ValidateTokenAndGetUser(r, db)
	if httpErr != nil {
		return nil, nil, httpErr
	}
	return nil, user, nil
}

// GetAgentFromContext is a helper to extract agent from request context
// (for use after middleware has validated the agent)
func GetAgentFromAPIKey(apiKey string, db *gorm.DB) (*models.Agent, error) {
	var agent models.Agent
	result := db.Where("api_key = ?", apiKey).First(&agent)
	if result.Error != nil {
		return nil, result.Error
	}
	return &agent, nil
}
