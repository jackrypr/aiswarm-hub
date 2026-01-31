package models

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// Agent represents an AI agent that can participate in prediction markets
type Agent struct {
	gorm.Model
	ID          int64  `json:"id" gorm:"primary_key"`
	Name        string `json:"name" gorm:"unique;not null;size:50"`
	Description string `json:"description" gorm:"size:500"`

	// Authentication
	APIKey string `json:"apiKey,omitempty" gorm:"unique;not null"`

	// Ownership - human who claimed this agent
	OwnerUserID *int64 `json:"ownerUserId,omitempty"`
	ClaimToken  string `json:"-" gorm:"unique"` // Used for claim verification
	ClaimedAt   *time.Time `json:"claimedAt,omitempty"`

	// Reputation System
	Reputation         float64 `json:"reputation" gorm:"default:0.5"`
	TotalPredictions   int64   `json:"totalPredictions" gorm:"default:0"`
	CorrectPredictions int64   `json:"correctPredictions" gorm:"default:0"`
	TotalWagered       int64   `json:"totalWagered" gorm:"default:0"`
	TotalWon           int64   `json:"totalWon" gorm:"default:0"`

	// Status
	IsClaimed bool `json:"isClaimed" gorm:"default:false"`
	IsActive  bool `json:"isActive" gorm:"default:true"`

	// Profile
	AvatarURL     string `json:"avatarUrl,omitempty" gorm:"size:500"`
	FrameworkType string `json:"frameworkType,omitempty" gorm:"size:50"` // "openclaw", "langchain", "autogen", "custom"
	PersonalEmoji string `json:"personalEmoji,omitempty" gorm:"size:10"`

	// Virtual balance for the agent
	AccountBalance int64 `json:"accountBalance" gorm:"default:10000"`
}

// AgentPublic is the public-facing agent profile
type AgentPublic struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Reputation         float64 `json:"reputation"`
	TotalPredictions   int64   `json:"totalPredictions"`
	CorrectPredictions int64   `json:"correctPredictions"`
	IsClaimed          bool    `json:"isClaimed"`
	IsActive           bool    `json:"isActive"`
	AvatarURL          string  `json:"avatarUrl,omitempty"`
	FrameworkType      string  `json:"frameworkType,omitempty"`
	PersonalEmoji      string  `json:"personalEmoji,omitempty"`
}

// AgentRegistration is the response when registering a new agent
type AgentRegistration struct {
	Agent            AgentPublic `json:"agent"`
	APIKey           string      `json:"apiKey"`
	ClaimURL         string      `json:"claimUrl"`
	VerificationCode string      `json:"verificationCode"`
	Important        string      `json:"important"`
}

// ToPublic converts Agent to AgentPublic (hides sensitive fields)
func (a *Agent) ToPublic() AgentPublic {
	return AgentPublic{
		ID:                 a.ID,
		Name:               a.Name,
		Description:        a.Description,
		Reputation:         a.Reputation,
		TotalPredictions:   a.TotalPredictions,
		CorrectPredictions: a.CorrectPredictions,
		IsClaimed:          a.IsClaimed,
		IsActive:           a.IsActive,
		AvatarURL:          a.AvatarURL,
		FrameworkType:      a.FrameworkType,
		PersonalEmoji:      a.PersonalEmoji,
	}
}

// GenerateAPIKey creates a secure random API key for an agent
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "swarm_sk_" + hex.EncodeToString(bytes), nil
}

// GenerateClaimToken creates a token for claim verification
func GenerateClaimToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "swarm_claim_" + hex.EncodeToString(bytes), nil
}

// GenerateVerificationCode creates a human-readable verification code
func GenerateVerificationCode() (string, error) {
	adjectives := []string{"swift", "clever", "bright", "keen", "sharp", "wise", "bold", "calm"}
	nouns := []string{"fox", "owl", "hawk", "wolf", "bear", "lion", "eagle", "raven"}

	adjBytes := make([]byte, 1)
	nounBytes := make([]byte, 1)
	numBytes := make([]byte, 2)

	if _, err := rand.Read(adjBytes); err != nil {
		return "", err
	}
	if _, err := rand.Read(nounBytes); err != nil {
		return "", err
	}
	if _, err := rand.Read(numBytes); err != nil {
		return "", err
	}

	adj := adjectives[int(adjBytes[0])%len(adjectives)]
	noun := nouns[int(nounBytes[0])%len(nouns)]
	num := hex.EncodeToString(numBytes)

	return adj + "-" + noun + "-" + num, nil
}

// UpdateReputation recalculates the agent's reputation based on prediction history
func (a *Agent) UpdateReputation() {
	if a.TotalPredictions == 0 {
		a.Reputation = 0.5 // Default for new agents
		return
	}

	// Base accuracy
	accuracy := float64(a.CorrectPredictions) / float64(a.TotalPredictions)

	// Bayesian smoothing with prior of 0.5 and strength of 10
	// This prevents wild swings with few predictions
	priorStrength := 10.0
	smoothedAccuracy := (accuracy*float64(a.TotalPredictions) + 0.5*priorStrength) / (float64(a.TotalPredictions) + priorStrength)

	a.Reputation = smoothedAccuracy
}

// CalculateWeight returns the voting weight for this agent
// Based on reputation and number of predictions (experience)
func (a *Agent) CalculateWeight() float64 {
	// Experience factor: sqrt of total predictions, capped at 10
	experienceFactor := 1.0
	if a.TotalPredictions > 0 {
		experienceFactor = min(10.0, 1.0+float64(a.TotalPredictions)/100.0)
	}

	return a.Reputation * experienceFactor
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
