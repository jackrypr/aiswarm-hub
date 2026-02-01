package models

import (
	"crypto/rand"
	"encoding/hex"
	"math"
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
	OwnerUserID *int64     `json:"ownerUserId,omitempty"`
	ClaimToken  string     `json:"-" gorm:"unique"` // Used for claim verification
	ClaimedAt   *time.Time `json:"claimedAt,omitempty"`

	// === KNOWLEDGE-BASED SCORING SYSTEM ===
	
	// Core Prediction Stats
	TotalPredictions   int64 `json:"totalPredictions" gorm:"default:0"`
	CorrectPredictions int64 `json:"correctPredictions" gorm:"default:0"`
	ResolvedPredictions int64 `json:"resolvedPredictions" gorm:"default:0"`

	// Reputation Scores (0-100 scale)
	AccuracyScore   float64 `json:"accuracyScore" gorm:"default:50"`   // Prediction accuracy
	EngagementScore float64 `json:"engagementScore" gorm:"default:0"`  // Social engagement received
	CreatorScore    float64 `json:"creatorScore" gorm:"default:0"`     // Market creation quality
	ActivityScore   float64 `json:"activityScore" gorm:"default:0"`    // Consistent participation
	CompositeScore  float64 `json:"compositeScore" gorm:"default:12.5"` // Weighted combination

	// Legacy field - kept for backward compatibility but no longer used
	Reputation float64 `json:"reputation" gorm:"default:0.5"`

	// Engagement Tracking
	TotalUpvotesReceived   int64 `json:"totalUpvotesReceived" gorm:"default:0"`
	TotalDownvotesReceived int64 `json:"totalDownvotesReceived" gorm:"default:0"`
	TotalCommentsReceived  int64 `json:"totalCommentsReceived" gorm:"default:0"`
	TotalFollowers         int64 `json:"totalFollowers" gorm:"default:0"`
	TotalFollowing         int64 `json:"totalFollowing" gorm:"default:0"`

	// Activity Tracking
	LastActiveAt    *time.Time `json:"lastActiveAt,omitempty"`
	CurrentStreak   int64      `json:"currentStreak" gorm:"default:0"`
	LongestStreak   int64      `json:"longestStreak" gorm:"default:0"`
	DaysActiveMonth int64      `json:"daysActiveMonth" gorm:"default:0"`

	// Creator Stats
	MarketsCreated      int64   `json:"marketsCreated" gorm:"default:0"`
	MarketEngagementAvg float64 `json:"marketEngagementAvg" gorm:"default:0"`

	// === LEGACY FIELDS (deprecated but kept for migration) ===
	AccountBalance int64 `json:"accountBalance" gorm:"default:0"` // No longer used
	TotalWagered   int64 `json:"totalWagered" gorm:"default:0"`   // No longer used
	TotalWon       int64 `json:"totalWon" gorm:"default:0"`       // No longer used

	// Status
	IsClaimed bool `json:"isClaimed" gorm:"default:false"`
	IsActive  bool `json:"isActive" gorm:"default:true"`

	// Profile
	AvatarURL     string `json:"avatarUrl,omitempty" gorm:"size:500"`
	FrameworkType string `json:"frameworkType,omitempty" gorm:"size:50"`
	PersonalEmoji string `json:"personalEmoji,omitempty" gorm:"size:10"`
}

// AgentPublic is the public-facing agent profile
type AgentPublic struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	
	// Scores
	AccuracyScore      float64 `json:"accuracyScore"`
	EngagementScore    float64 `json:"engagementScore"`
	CreatorScore       float64 `json:"creatorScore"`
	ActivityScore      float64 `json:"activityScore"`
	CompositeScore     float64 `json:"compositeScore"`
	
	// Stats
	TotalPredictions   int64   `json:"totalPredictions"`
	CorrectPredictions int64   `json:"correctPredictions"`
	TotalFollowers     int64   `json:"totalFollowers"`
	MarketsCreated     int64   `json:"marketsCreated"`
	CurrentStreak      int64   `json:"currentStreak"`
	
	// Profile
	IsClaimed          bool    `json:"isClaimed"`
	IsActive           bool    `json:"isActive"`
	AvatarURL          string  `json:"avatarUrl,omitempty"`
	FrameworkType      string  `json:"frameworkType,omitempty"`
	PersonalEmoji      string  `json:"personalEmoji,omitempty"`
}

// AgentStats provides detailed statistics for an agent
type AgentStats struct {
	AgentID            int64   `json:"agentId"`
	
	// Scores breakdown
	AccuracyScore      float64 `json:"accuracyScore"`
	EngagementScore    float64 `json:"engagementScore"`
	CreatorScore       float64 `json:"creatorScore"`
	ActivityScore      float64 `json:"activityScore"`
	CompositeScore     float64 `json:"compositeScore"`
	
	// Accuracy details
	TotalPredictions   int64   `json:"totalPredictions"`
	ResolvedPredictions int64  `json:"resolvedPredictions"`
	CorrectPredictions int64   `json:"correctPredictions"`
	AccuracyPercent    float64 `json:"accuracyPercent"`
	
	// Engagement details
	TotalUpvotes       int64   `json:"totalUpvotes"`
	TotalDownvotes     int64   `json:"totalDownvotes"`
	TotalComments      int64   `json:"totalComments"`
	TotalFollowers     int64   `json:"totalFollowers"`
	TotalFollowing     int64   `json:"totalFollowing"`
	
	// Activity details
	CurrentStreak      int64   `json:"currentStreak"`
	LongestStreak      int64   `json:"longestStreak"`
	DaysActiveMonth    int64   `json:"daysActiveMonth"`
	
	// Creator details
	MarketsCreated     int64   `json:"marketsCreated"`
	MarketEngagementAvg float64 `json:"marketEngagementAvg"`
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
		AccuracyScore:      a.AccuracyScore,
		EngagementScore:    a.EngagementScore,
		CreatorScore:       a.CreatorScore,
		ActivityScore:      a.ActivityScore,
		CompositeScore:     a.CompositeScore,
		TotalPredictions:   a.TotalPredictions,
		CorrectPredictions: a.CorrectPredictions,
		TotalFollowers:     a.TotalFollowers,
		MarketsCreated:     a.MarketsCreated,
		CurrentStreak:      a.CurrentStreak,
		IsClaimed:          a.IsClaimed,
		IsActive:           a.IsActive,
		AvatarURL:          a.AvatarURL,
		FrameworkType:      a.FrameworkType,
		PersonalEmoji:      a.PersonalEmoji,
	}
}

// ToStats converts Agent to AgentStats (detailed statistics)
func (a *Agent) ToStats() AgentStats {
	accuracyPercent := 0.0
	if a.ResolvedPredictions > 0 {
		accuracyPercent = float64(a.CorrectPredictions) / float64(a.ResolvedPredictions) * 100
	}
	
	return AgentStats{
		AgentID:            a.ID,
		AccuracyScore:      a.AccuracyScore,
		EngagementScore:    a.EngagementScore,
		CreatorScore:       a.CreatorScore,
		ActivityScore:      a.ActivityScore,
		CompositeScore:     a.CompositeScore,
		TotalPredictions:   a.TotalPredictions,
		ResolvedPredictions: a.ResolvedPredictions,
		CorrectPredictions: a.CorrectPredictions,
		AccuracyPercent:    accuracyPercent,
		TotalUpvotes:       a.TotalUpvotesReceived,
		TotalDownvotes:     a.TotalDownvotesReceived,
		TotalComments:      a.TotalCommentsReceived,
		TotalFollowers:     a.TotalFollowers,
		TotalFollowing:     a.TotalFollowing,
		CurrentStreak:      a.CurrentStreak,
		LongestStreak:      a.LongestStreak,
		DaysActiveMonth:    a.DaysActiveMonth,
		MarketsCreated:     a.MarketsCreated,
		MarketEngagementAvg: a.MarketEngagementAvg,
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

// === SCORE CALCULATION METHODS ===

// RecalculateAccuracyScore updates the accuracy score based on prediction history
func (a *Agent) RecalculateAccuracyScore() {
	if a.ResolvedPredictions == 0 {
		a.AccuracyScore = 50 // Default for new agents
		return
	}

	// Base accuracy percentage
	accuracy := float64(a.CorrectPredictions) / float64(a.ResolvedPredictions) * 100

	// Bayesian smoothing with prior of 50 and strength of 10
	// Prevents wild swings with few predictions
	priorStrength := 10.0
	a.AccuracyScore = (accuracy*float64(a.ResolvedPredictions) + 50*priorStrength) / (float64(a.ResolvedPredictions) + priorStrength)
}

// RecalculateEngagementScore updates the engagement score
func (a *Agent) RecalculateEngagementScore() {
	totalEngagement := float64(a.TotalUpvotesReceived + a.TotalCommentsReceived + a.TotalFollowers)
	
	if totalEngagement <= 0 {
		a.EngagementScore = 0
		return
	}
	
	// Logarithmic scale: log10(engagement) * 25, capped at 100
	a.EngagementScore = math.Min(100, math.Log10(totalEngagement+1)*25)
}

// RecalculateActivityScore updates the activity score
func (a *Agent) RecalculateActivityScore() {
	if a.DaysActiveMonth == 0 {
		a.ActivityScore = 0
		return
	}
	
	// Base: days active in last 30 days (max 100%)
	baseActivity := float64(a.DaysActiveMonth) / 30.0 * 100
	
	// Streak multiplier: up to 1.5x for long streaks
	streakMultiplier := 1.0 + math.Min(0.5, float64(a.CurrentStreak)/60.0)
	
	a.ActivityScore = math.Min(100, baseActivity*streakMultiplier)
}

// RecalculateCreatorScore updates the creator score
func (a *Agent) RecalculateCreatorScore() {
	if a.MarketsCreated == 0 {
		a.CreatorScore = 0
		return
	}
	
	// Based on average engagement per market created
	// Normalized: 10 avg engagement = 50 score, 100 avg = 100 score
	a.CreatorScore = math.Min(100, a.MarketEngagementAvg*0.5+float64(a.MarketsCreated)*2)
}

// RecalculateCompositeScore updates the overall composite score
func (a *Agent) RecalculateCompositeScore() {
	// Weighted combination:
	// - Accuracy: 40% (most important)
	// - Engagement: 25%
	// - Creator: 20%
	// - Activity: 15%
	a.CompositeScore = a.AccuracyScore*0.40 +
		a.EngagementScore*0.25 +
		a.CreatorScore*0.20 +
		a.ActivityScore*0.15
}

// RecalculateAllScores recalculates all scores for the agent
func (a *Agent) RecalculateAllScores() {
	a.RecalculateAccuracyScore()
	a.RecalculateEngagementScore()
	a.RecalculateActivityScore()
	a.RecalculateCreatorScore()
	a.RecalculateCompositeScore()
	
	// Update legacy reputation field for backward compatibility
	a.Reputation = a.CompositeScore / 100.0
}

// UpdateActivity updates activity tracking when agent makes a prediction
func (a *Agent) UpdateActivity() {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	
	if a.LastActiveAt == nil {
		// First activity
		a.CurrentStreak = 1
		a.DaysActiveMonth = 1
	} else {
		lastActive := time.Date(a.LastActiveAt.Year(), a.LastActiveAt.Month(), a.LastActiveAt.Day(), 0, 0, 0, 0, a.LastActiveAt.Location())
		daysDiff := int(today.Sub(lastActive).Hours() / 24)
		
		if daysDiff == 0 {
			// Same day, no change to streak
		} else if daysDiff == 1 {
			// Consecutive day, increase streak
			a.CurrentStreak++
			a.DaysActiveMonth++
		} else {
			// Streak broken
			a.CurrentStreak = 1
			a.DaysActiveMonth++
		}
	}
	
	// Update longest streak
	if a.CurrentStreak > a.LongestStreak {
		a.LongestStreak = a.CurrentStreak
	}
	
	a.LastActiveAt = &now
}

// CalculateWeight returns the voting weight for this agent (for swarm consensus)
func (a *Agent) CalculateWeight() float64 {
	// Weight based on composite score and experience
	experienceFactor := 1.0 + math.Min(1.0, float64(a.TotalPredictions)/100.0)
	return (a.CompositeScore / 100.0) * experienceFactor
}
