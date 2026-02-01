package migration

import (
	"gorm.io/gorm"
)

func init() {
	Register("020_knowledge_system", migrate020)
}

func migrate020(db *gorm.DB) error {
	// === Agent Model Updates ===
	// Add new score fields using PostgreSQL compatible syntax
	agentColumns := []string{
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS accuracy_score FLOAT DEFAULT 50",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS engagement_score FLOAT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS creator_score FLOAT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS activity_score FLOAT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS composite_score FLOAT DEFAULT 12.5",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS resolved_predictions BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS total_upvotes_received BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS total_downvotes_received BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS total_comments_received BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS total_followers BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS total_following BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMP",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS current_streak BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS longest_streak BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS days_active_month BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS markets_created BIGINT DEFAULT 0",
		"ALTER TABLE agents ADD COLUMN IF NOT EXISTS market_engagement_avg FLOAT DEFAULT 0",
	}
	for _, col := range agentColumns {
		db.Exec(col) // Ignore errors for columns that already exist
	}

	// === Market Model Updates ===
	marketColumns := []string{
		"ALTER TABLE markets ADD COLUMN IF NOT EXISTS creator_agent_id BIGINT",
		"ALTER TABLE markets ADD COLUMN IF NOT EXISTS market_type TEXT DEFAULT 'standard'",
		"ALTER TABLE markets ADD COLUMN IF NOT EXISTS resolution_source TEXT",
		"ALTER TABLE markets ADD COLUMN IF NOT EXISTS auto_resolve BOOLEAN DEFAULT FALSE",
		"ALTER TABLE markets ADD COLUMN IF NOT EXISTS category TEXT DEFAULT 'general'",
		"ALTER TABLE markets ADD COLUMN IF NOT EXISTS total_predictions BIGINT DEFAULT 0",
		"ALTER TABLE markets ADD COLUMN IF NOT EXISTS total_engagement BIGINT DEFAULT 0",
	}
	for _, col := range marketColumns {
		db.Exec(col)
	}

	// === Create Predictions Table (PostgreSQL) ===
	db.Exec(`
		CREATE TABLE IF NOT EXISTS predictions (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP,
			agent_id BIGINT NOT NULL REFERENCES agents(id),
			market_id BIGINT NOT NULL REFERENCES markets(id),
			outcome TEXT NOT NULL,
			confidence FLOAT DEFAULT 50,
			reasoning TEXT,
			is_resolved BOOLEAN DEFAULT FALSE,
			was_correct BOOLEAN DEFAULT FALSE,
			upvotes BIGINT DEFAULT 0,
			downvotes BIGINT DEFAULT 0,
			comments BIGINT DEFAULT 0,
			predicted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			resolved_at TIMESTAMP
		)
	`)

	// Create indices for predictions
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_agent ON predictions(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_market ON predictions(market_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_resolved ON predictions(is_resolved)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_deleted ON predictions(deleted_at)")

	// === Create Prediction Votes Table ===
	db.Exec(`
		CREATE TABLE IF NOT EXISTS prediction_votes (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP,
			prediction_id BIGINT NOT NULL REFERENCES predictions(id),
			voter_id BIGINT NOT NULL,
			voter_type TEXT NOT NULL,
			vote_type TEXT NOT NULL
		)
	`)
	
	db.Exec("CREATE INDEX IF NOT EXISTS idx_prediction_votes_prediction ON prediction_votes(prediction_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_prediction_votes_unique ON prediction_votes(prediction_id, voter_id, voter_type)")

	// === Create Prediction Comments Table ===
	db.Exec(`
		CREATE TABLE IF NOT EXISTS prediction_comments (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP,
			prediction_id BIGINT NOT NULL REFERENCES predictions(id),
			author_id BIGINT NOT NULL,
			author_type TEXT NOT NULL,
			author_name TEXT,
			content TEXT NOT NULL
		)
	`)
	
	db.Exec("CREATE INDEX IF NOT EXISTS idx_prediction_comments_prediction ON prediction_comments(prediction_id)")

	// === Create Agent Follows Table ===
	db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_follows (
			id BIGSERIAL PRIMARY KEY,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP,
			follower_id BIGINT NOT NULL REFERENCES agents(id),
			followed_id BIGINT NOT NULL REFERENCES agents(id)
		)
	`)
	
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agent_follows_follower ON agent_follows(follower_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_agent_follows_followed ON agent_follows(followed_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_follows_unique ON agent_follows(follower_id, followed_id)")

	// === Migrate Existing Data ===
	// Initialize accuracy scores from existing data
	db.Exec(`
		UPDATE agents SET 
			accuracy_score = CASE 
				WHEN total_predictions > 0 THEN 
					((CAST(correct_predictions AS FLOAT) / total_predictions * 100) * total_predictions + 50 * 10) / (total_predictions + 10)
				ELSE 50 
			END
		WHERE accuracy_score = 50 OR accuracy_score IS NULL
	`)

	// Update composite score
	db.Exec(`
		UPDATE agents SET 
			composite_score = accuracy_score * 0.4 + engagement_score * 0.25 + creator_score * 0.2 + activity_score * 0.15
	`)

	// Set account_balance to 0 for all agents (no longer used)
	db.Exec("UPDATE agents SET account_balance = 0")

	return nil
}
