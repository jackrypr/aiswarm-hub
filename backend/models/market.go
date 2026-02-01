package models

import (
	"time"

	"gorm.io/gorm"
)

type Market struct {
	gorm.Model
	ID                      int64     `json:"id" gorm:"primary_key"`
	QuestionTitle           string    `json:"questionTitle" gorm:"not null"`
	Description             string    `json:"description" gorm:"not null"`
	OutcomeType             string    `json:"outcomeType" gorm:"not null"`
	ResolutionDateTime      time.Time `json:"resolutionDateTime" gorm:"not null"`
	FinalResolutionDateTime time.Time `json:"finalResolutionDateTime"`
	UTCOffset               int       `json:"utcOffset"`
	IsResolved              bool      `json:"isResolved"`
	ResolutionResult        string    `json:"resolutionResult"`
	InitialProbability      float64   `json:"initialProbability" gorm:"not null"`
	YesLabel                string    `json:"yesLabel" gorm:"default:YES"`
	NoLabel                 string    `json:"noLabel" gorm:"default:NO"`
	CreatorUsername         string    `json:"creatorUsername" gorm:"not null"`
	Creator                 User      `gorm:"foreignKey:CreatorUsername;references:Username"`
	
	// === NEW: Knowledge System Fields ===
	
	// Creator tracking (for agent-created markets)
	CreatorAgentID  *int64 `json:"creatorAgentId,omitempty" gorm:"index"`
	
	// Market type for real-time/daily predictions
	MarketType       string `json:"marketType" gorm:"default:standard"`  // "standard", "realtime", "daily"
	
	// Auto-resolution for real-time markets
	ResolutionSource string `json:"resolutionSource,omitempty"` // API endpoint for auto-resolution
	AutoResolve      bool   `json:"autoResolve" gorm:"default:false"`
	
	// Category for filtering
	Category         string `json:"category" gorm:"default:general;index"` // politics, crypto, sports, etc.
	
	// Engagement stats
	TotalPredictions int64  `json:"totalPredictions" gorm:"default:0"`
	TotalEngagement  int64  `json:"totalEngagement" gorm:"default:0"`  // upvotes + comments on predictions
}
