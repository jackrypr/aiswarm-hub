package models

import (
	"time"

	"gorm.io/gorm"
)

// Prediction represents an agent's prediction on a market
// Replaces the old "bet" system - no money involved, just knowledge
type Prediction struct {
	gorm.Model
	ID       int64 `json:"id" gorm:"primary_key"`
	AgentID  int64 `json:"agentId" gorm:"not null;index"`
	MarketID int64 `json:"marketId" gorm:"not null;index"`

	// Prediction details
	Outcome    string  `json:"outcome" gorm:"not null;size:10"`  // "YES" or "NO"
	Confidence float64 `json:"confidence" gorm:"default:50"`     // 0-100 confidence level
	Reasoning  string  `json:"reasoning" gorm:"size:2000"`       // Why this prediction

	// Resolution
	IsResolved bool `json:"isResolved" gorm:"default:false;index"`
	WasCorrect bool `json:"wasCorrect" gorm:"default:false"`

	// Engagement stats
	Upvotes   int64 `json:"upvotes" gorm:"default:0"`
	Downvotes int64 `json:"downvotes" gorm:"default:0"`
	Comments  int64 `json:"comments" gorm:"default:0"`

	// Timestamps
	PredictedAt time.Time  `json:"predictedAt" gorm:"not null"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`

	// Relations (for preloading)
	Agent  *Agent  `json:"agent,omitempty" gorm:"foreignKey:AgentID"`
	Market *Market `json:"market,omitempty" gorm:"foreignKey:MarketID"`
}

// PredictionPublic is the public-facing prediction
type PredictionPublic struct {
	ID          int64     `json:"id"`
	AgentID     int64     `json:"agentId"`
	AgentName   string    `json:"agentName,omitempty"`
	MarketID    int64     `json:"marketId"`
	MarketTitle string    `json:"marketTitle,omitempty"`
	Outcome     string    `json:"outcome"`
	Confidence  float64   `json:"confidence"`
	Reasoning   string    `json:"reasoning,omitempty"`
	IsResolved  bool      `json:"isResolved"`
	WasCorrect  bool      `json:"wasCorrect"`
	Upvotes     int64     `json:"upvotes"`
	Downvotes   int64     `json:"downvotes"`
	Comments    int64     `json:"comments"`
	PredictedAt time.Time `json:"predictedAt"`
	ResolvedAt  *time.Time `json:"resolvedAt,omitempty"`
}

// PredictionRequest is the request body for making a prediction
type PredictionRequest struct {
	MarketID   int64   `json:"marketId" binding:"required"`
	Outcome    string  `json:"outcome" binding:"required"`  // "YES" or "NO"
	Confidence float64 `json:"confidence"`                  // 0-100, optional
	Reasoning  string  `json:"reasoning"`                   // optional but encouraged
}

// PredictionResponse is the response after making a prediction
type PredictionResponse struct {
	Success    bool             `json:"success"`
	Prediction PredictionPublic `json:"prediction"`
	Message    string           `json:"message"`
}

// ToPublic converts Prediction to PredictionPublic
func (p *Prediction) ToPublic() PredictionPublic {
	pub := PredictionPublic{
		ID:          p.ID,
		AgentID:     p.AgentID,
		MarketID:    p.MarketID,
		Outcome:     p.Outcome,
		Confidence:  p.Confidence,
		Reasoning:   p.Reasoning,
		IsResolved:  p.IsResolved,
		WasCorrect:  p.WasCorrect,
		Upvotes:     p.Upvotes,
		Downvotes:   p.Downvotes,
		Comments:    p.Comments,
		PredictedAt: p.PredictedAt,
		ResolvedAt:  p.ResolvedAt,
	}

	if p.Agent != nil {
		pub.AgentName = p.Agent.Name
	}
	if p.Market != nil {
		pub.MarketTitle = p.Market.QuestionTitle
	}

	return pub
}

// PredictionVote represents a vote on a prediction
type PredictionVote struct {
	gorm.Model
	ID           int64  `json:"id" gorm:"primary_key"`
	PredictionID int64  `json:"predictionId" gorm:"not null;index"`
	VoterID      int64  `json:"voterId" gorm:"not null;index"`      // Agent or User ID
	VoterType    string `json:"voterType" gorm:"not null;size:10"`  // "agent" or "user"
	VoteType     string `json:"voteType" gorm:"not null;size:10"`   // "up" or "down"
}

// VoteRequest is the request body for voting
type VoteRequest struct {
	VoteType string `json:"voteType" binding:"required"` // "up" or "down"
}

// PredictionComment represents a comment on a prediction
type PredictionComment struct {
	gorm.Model
	ID           int64  `json:"id" gorm:"primary_key"`
	PredictionID int64  `json:"predictionId" gorm:"not null;index"`
	AuthorID     int64  `json:"authorId" gorm:"not null;index"`
	AuthorType   string `json:"authorType" gorm:"not null;size:10"` // "agent" or "user"
	AuthorName   string `json:"authorName" gorm:"size:50"`
	Content      string `json:"content" gorm:"not null;size:1000"`
}

// CommentRequest is the request body for commenting
type CommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// AgentFollow represents a follow relationship between agents
type AgentFollow struct {
	gorm.Model
	ID         int64 `json:"id" gorm:"primary_key"`
	FollowerID int64 `json:"followerId" gorm:"not null;index;uniqueIndex:idx_follow"`
	FollowedID int64 `json:"followedId" gorm:"not null;index;uniqueIndex:idx_follow"`
}

// LeaderboardEntry represents an entry in the leaderboard
type LeaderboardEntry struct {
	Rank              int64   `json:"rank"`
	AgentID           int64   `json:"agentId"`
	AgentName         string  `json:"agentName"`
	AvatarURL         string  `json:"avatarUrl,omitempty"`
	PersonalEmoji     string  `json:"personalEmoji,omitempty"`
	CompositeScore    float64 `json:"compositeScore"`
	AccuracyScore     float64 `json:"accuracyScore"`
	EngagementScore   float64 `json:"engagementScore"`
	CreatorScore      float64 `json:"creatorScore"`
	ActivityScore     float64 `json:"activityScore"`
	TotalPredictions  int64   `json:"totalPredictions"`
	CorrectPredictions int64  `json:"correctPredictions"`
	CurrentStreak     int64   `json:"currentStreak"`
}

// LeaderboardResponse is the response for leaderboard endpoints
type LeaderboardResponse struct {
	Leaderboard []LeaderboardEntry `json:"leaderboard"`
	TotalAgents int64              `json:"totalAgents"`
	SortBy      string             `json:"sortBy"`
	Page        int                `json:"page"`
	PageSize    int                `json:"pageSize"`
}
