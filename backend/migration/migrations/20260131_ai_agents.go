package migrations

import (
	"log"
	"time"

	"gorm.io/gorm"
	"socialpredict/migration"
)

func init() {
	if err := migration.Register("20260131_ai_agents", Migration20260131AIAgents); err != nil {
		log.Fatalf("Failed to register migration 20260131_ai_agents: %v", err)
	}
}

// Agent model for migration
type Agent struct {
	gorm.Model
	ID          int64  `gorm:"primary_key"`
	Name        string `gorm:"unique;not null;size:50"`
	Description string `gorm:"size:500"`

	// Authentication
	APIKey string `gorm:"unique;not null"`

	// Ownership
	OwnerUserID *int64
	ClaimToken  string `gorm:"unique"`
	ClaimedAt   *time.Time

	// Reputation System
	Reputation         float64 `gorm:"default:0.5"`
	TotalPredictions   int64   `gorm:"default:0"`
	CorrectPredictions int64   `gorm:"default:0"`
	TotalWagered       int64   `gorm:"default:0"`
	TotalWon           int64   `gorm:"default:0"`

	// Status
	IsClaimed bool `gorm:"default:false"`
	IsActive  bool `gorm:"default:true"`

	// Profile
	AvatarURL     string `gorm:"size:500"`
	FrameworkType string `gorm:"size:50"`
	PersonalEmoji string `gorm:"size:10"`

	// Balance
	AccountBalance int64 `gorm:"default:10000"`
}

// AgentBet model for migration
type AgentBet struct {
	gorm.Model
	ID             int64   `gorm:"primary_key"`
	AgentID        int64   `gorm:"not null;index"`
	MarketID       int64   `gorm:"not null;index"`
	Amount         int64   `gorm:"not null"`
	Outcome        string  `gorm:"not null"`
	Confidence     float64 `gorm:"default:0.5"`
	Reasoning      string  `gorm:"size:1000"`
	PlacedAt       time.Time
	SharesReceived float64
	AveragePrice   float64
}

// Migration20260131AIAgents creates the AI agent tables
func Migration20260131AIAgents(db *gorm.DB) error {
	// Create agents table
	if err := db.AutoMigrate(&Agent{}); err != nil {
		return err
	}

	// Create agent_bets table
	if err := db.AutoMigrate(&AgentBet{}); err != nil {
		return err
	}

	// Add indexes for performance
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_reputation ON agents(reputation DESC)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agents_name ON agents(name)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agent_bets_market ON agent_bets(market_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agent_bets_agent ON agent_bets(agent_id)")

	return nil
}

// Rollback20260131AIAgents removes the AI agent tables
func Rollback20260131AIAgents(db *gorm.DB) error {
	if err := db.Migrator().DropTable(&AgentBet{}); err != nil {
		return err
	}
	if err := db.Migrator().DropTable(&Agent{}); err != nil {
		return err
	}
	return nil
}

// TableName specifies the table name for Agent
func (Agent) TableName() string {
	return "agents"
}

// TableName specifies the table name for AgentBet
func (AgentBet) TableName() string {
	return "agent_bets"
}
