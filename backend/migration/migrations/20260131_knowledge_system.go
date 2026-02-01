package migrations

import (
	"log"
	"time"

	"gorm.io/gorm"
	"socialpredict/migration"
)

func init() {
	if err := migration.Register("20260131_knowledge_system", Migration20260131KnowledgeSystem); err != nil {
		log.Fatalf("Failed to register migration 20260131_knowledge_system: %v", err)
	}
}

// Prediction model for migration - replaces balance-based betting
type Prediction struct {
	gorm.Model
	ID          int64     `gorm:"primary_key"`
	AgentID     int64     `gorm:"not null;index"`
	MarketID    int64     `gorm:"not null;index"`
	Outcome     string    `gorm:"not null;size:10"`
	Confidence  float64   `gorm:"default:50"`
	Reasoning   string    `gorm:"size:2000"`
	IsResolved  bool      `gorm:"default:false;index"`
	WasCorrect  bool      `gorm:"default:false"`
	Upvotes     int64     `gorm:"default:0"`
	Downvotes   int64     `gorm:"default:0"`
	Comments    int64     `gorm:"default:0"`
	PredictedAt time.Time `gorm:"not null"`
	ResolvedAt  *time.Time
}

// PredictionVote model for migration
type PredictionVote struct {
	gorm.Model
	ID           int64  `gorm:"primary_key"`
	PredictionID int64  `gorm:"not null;index"`
	VoterID      int64  `gorm:"not null"`
	VoterType    string `gorm:"not null;size:20"`
	VoteType     string `gorm:"not null;size:10"`
}

// PredictionComment model for migration
type PredictionComment struct {
	gorm.Model
	ID           int64  `gorm:"primary_key"`
	PredictionID int64  `gorm:"not null;index"`
	AuthorID     int64  `gorm:"not null"`
	AuthorType   string `gorm:"not null;size:20"`
	AuthorName   string `gorm:"size:100"`
	Content      string `gorm:"not null;size:2000"`
}

// AgentFollow model for migration
type AgentFollow struct {
	gorm.Model
	ID         int64 `gorm:"primary_key"`
	FollowerID int64 `gorm:"not null;index"`
	FollowedID int64 `gorm:"not null;index"`
}

// Migration20260131KnowledgeSystem adds knowledge-based scoring and predictions
func Migration20260131KnowledgeSystem(db *gorm.DB) error {
	// === Create new tables ===
	
	// Create predictions table
	if err := db.AutoMigrate(&Prediction{}); err != nil {
		return err
	}

	// Create prediction_votes table
	if err := db.AutoMigrate(&PredictionVote{}); err != nil {
		return err
	}

	// Create prediction_comments table
	if err := db.AutoMigrate(&PredictionComment{}); err != nil {
		return err
	}

	// Create agent_follows table
	if err := db.AutoMigrate(&AgentFollow{}); err != nil {
		return err
	}

	// === Add indexes ===
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_agent ON predictions(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_market ON predictions(market_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_resolved ON predictions(is_resolved)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_prediction_votes_prediction ON prediction_votes(prediction_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_prediction_votes_unique ON prediction_votes(prediction_id, voter_id, voter_type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_prediction_comments_prediction ON prediction_comments(prediction_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agent_follows_follower ON agent_follows(follower_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agent_follows_followed ON agent_follows(followed_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_follows_unique ON agent_follows(follower_id, followed_id)")

	// === Add new columns to agents table ===
	agentColumns := []struct {
		name    string
		colType string
		defVal  string
	}{
		{"accuracy_score", "FLOAT", "50"},
		{"engagement_score", "FLOAT", "0"},
		{"creator_score", "FLOAT", "0"},
		{"activity_score", "FLOAT", "0"},
		{"composite_score", "FLOAT", "12.5"},
		{"resolved_predictions", "BIGINT", "0"},
		{"total_upvotes_received", "BIGINT", "0"},
		{"total_downvotes_received", "BIGINT", "0"},
		{"total_comments_received", "BIGINT", "0"},
		{"total_followers", "BIGINT", "0"},
		{"total_following", "BIGINT", "0"},
		{"current_streak", "BIGINT", "0"},
		{"longest_streak", "BIGINT", "0"},
		{"days_active_month", "BIGINT", "0"},
		{"markets_created", "BIGINT", "0"},
		{"market_engagement_avg", "FLOAT", "0"},
		{"last_active_at", "TIMESTAMP", "NULL"},
	}

	for _, col := range agentColumns {
		db.Exec("ALTER TABLE agents ADD COLUMN IF NOT EXISTS " + col.name + " " + col.colType + " DEFAULT " + col.defVal)
	}

	// === Add new columns to markets table ===
	marketColumns := []struct {
		name    string
		colType string
		defVal  string
	}{
		{"creator_agent_id", "BIGINT", "NULL"},
		{"market_type", "TEXT", "'standard'"},
		{"resolution_source", "TEXT", "NULL"},
		{"auto_resolve", "BOOLEAN", "FALSE"},
		{"category", "TEXT", "'general'"},
		{"total_predictions", "BIGINT", "0"},
		{"total_engagement", "BIGINT", "0"},
	}

	for _, col := range marketColumns {
		db.Exec("ALTER TABLE markets ADD COLUMN IF NOT EXISTS " + col.name + " " + col.colType + " DEFAULT " + col.defVal)
	}

	// === Migrate existing data ===
	// Initialize accuracy scores from existing agent data
	db.Exec(`
		UPDATE agents SET 
			accuracy_score = CASE 
				WHEN total_predictions > 0 THEN 
					((CAST(correct_predictions AS FLOAT) / total_predictions * 100) * total_predictions + 50 * 10) / (total_predictions + 10)
				ELSE 50 
			END
		WHERE accuracy_score IS NULL OR accuracy_score = 0
	`)

	// Update composite score
	db.Exec(`
		UPDATE agents SET 
			composite_score = COALESCE(accuracy_score, 50) * 0.4 + 
			                  COALESCE(engagement_score, 0) * 0.25 + 
			                  COALESCE(creator_score, 0) * 0.2 + 
			                  COALESCE(activity_score, 0) * 0.15
	`)

	// Set account_balance to 0 for all agents (no longer used)
	db.Exec("UPDATE agents SET account_balance = 0")

	return nil
}

// Rollback function if needed
func Rollback20260131KnowledgeSystem(db *gorm.DB) error {
	// Drop new tables
	db.Migrator().DropTable(&AgentFollow{})
	db.Migrator().DropTable(&PredictionComment{})
	db.Migrator().DropTable(&PredictionVote{})
	db.Migrator().DropTable(&Prediction{})
	return nil
}

// TableName specifies the table name for Prediction
func (Prediction) TableName() string {
	return "predictions"
}

// TableName specifies the table name for PredictionVote
func (PredictionVote) TableName() string {
	return "prediction_votes"
}

// TableName specifies the table name for PredictionComment
func (PredictionComment) TableName() string {
	return "prediction_comments"
}

// TableName specifies the table name for AgentFollow
func (AgentFollow) TableName() string {
	return "agent_follows"
}
