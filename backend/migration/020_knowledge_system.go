package migration

import (
	"gorm.io/gorm"
)

func init() {
	Register("020_knowledge_system", migrate020)
}

func migrate020(db *gorm.DB) error {
	// === Agent Model Updates ===
	
	// Add new score fields
	if err := db.Exec(`
		ALTER TABLE agents 
		ADD COLUMN IF NOT EXISTS accuracy_score REAL DEFAULT 50,
		ADD COLUMN IF NOT EXISTS engagement_score REAL DEFAULT 0,
		ADD COLUMN IF NOT EXISTS creator_score REAL DEFAULT 0,
		ADD COLUMN IF NOT EXISTS activity_score REAL DEFAULT 0,
		ADD COLUMN IF NOT EXISTS composite_score REAL DEFAULT 12.5,
		ADD COLUMN IF NOT EXISTS resolved_predictions INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS total_upvotes_received INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS total_downvotes_received INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS total_comments_received INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS total_followers INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS total_following INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMP,
		ADD COLUMN IF NOT EXISTS current_streak INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS longest_streak INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS days_active_month INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS markets_created INTEGER DEFAULT 0,
		ADD COLUMN IF NOT EXISTS market_engagement_avg REAL DEFAULT 0
	`).Error; err != nil {
		// SQLite doesn't support ADD COLUMN IF NOT EXISTS, try individual columns
		columns := []string{
			"ALTER TABLE agents ADD COLUMN accuracy_score REAL DEFAULT 50",
			"ALTER TABLE agents ADD COLUMN engagement_score REAL DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN creator_score REAL DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN activity_score REAL DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN composite_score REAL DEFAULT 12.5",
			"ALTER TABLE agents ADD COLUMN resolved_predictions INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN total_upvotes_received INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN total_downvotes_received INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN total_comments_received INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN total_followers INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN total_following INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN last_active_at TIMESTAMP",
			"ALTER TABLE agents ADD COLUMN current_streak INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN longest_streak INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN days_active_month INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN markets_created INTEGER DEFAULT 0",
			"ALTER TABLE agents ADD COLUMN market_engagement_avg REAL DEFAULT 0",
		}
		for _, col := range columns {
			db.Exec(col) // Ignore errors for columns that exist
		}
	}

	// === Market Model Updates ===
	
	marketColumns := []string{
		"ALTER TABLE markets ADD COLUMN creator_agent_id INTEGER",
		"ALTER TABLE markets ADD COLUMN market_type TEXT DEFAULT 'standard'",
		"ALTER TABLE markets ADD COLUMN resolution_source TEXT",
		"ALTER TABLE markets ADD COLUMN auto_resolve BOOLEAN DEFAULT FALSE",
		"ALTER TABLE markets ADD COLUMN category TEXT DEFAULT 'general'",
		"ALTER TABLE markets ADD COLUMN total_predictions INTEGER DEFAULT 0",
		"ALTER TABLE markets ADD COLUMN total_engagement INTEGER DEFAULT 0",
	}
	for _, col := range marketColumns {
		db.Exec(col) // Ignore errors for columns that exist
	}

	// === Create Predictions Table ===
	
	db.Exec(`
		CREATE TABLE IF NOT EXISTS predictions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			deleted_at TIMESTAMP,
			agent_id INTEGER NOT NULL,
			market_id INTEGER NOT NULL,
			outcome TEXT NOT NULL,
			confidence REAL DEFAULT 50,
			reasoning TEXT,
			is_resolved BOOLEAN DEFAULT FALSE,
			was_correct BOOLEAN DEFAULT FALSE,
			upvotes INTEGER DEFAULT 0,
			downvotes INTEGER DEFAULT 0,
			comments INTEGER DEFAULT 0,
			predicted_at TIMESTAMP NOT NULL,
			resolved_at TIMESTAMP,
			FOREIGN KEY (agent_id) REFERENCES agents(id),
			FOREIGN KEY (market_id) REFERENCES markets(id)
		)
	`)

	// Create indices for predictions
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_agent ON predictions(agent_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_market ON predictions(market_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_predictions_resolved ON predictions(is_resolved)")

	// === Create Prediction Votes Table ===
	
	db.Exec(`
		CREATE TABLE IF NOT EXISTS prediction_votes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			deleted_at TIMESTAMP,
			prediction_id INTEGER NOT NULL,
			voter_id INTEGER NOT NULL,
			voter_type TEXT NOT NULL,
			vote_type TEXT NOT NULL,
			FOREIGN KEY (prediction_id) REFERENCES predictions(id)
		)
	`)
	
	db.Exec("CREATE INDEX IF NOT EXISTS idx_prediction_votes_prediction ON prediction_votes(prediction_id)")
	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_prediction_votes_unique ON prediction_votes(prediction_id, voter_id, voter_type)")

	// === Create Prediction Comments Table ===
	
	db.Exec(`
		CREATE TABLE IF NOT EXISTS prediction_comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			deleted_at TIMESTAMP,
			prediction_id INTEGER NOT NULL,
			author_id INTEGER NOT NULL,
			author_type TEXT NOT NULL,
			author_name TEXT,
			content TEXT NOT NULL,
			FOREIGN KEY (prediction_id) REFERENCES predictions(id)
		)
	`)
	
	db.Exec("CREATE INDEX IF NOT EXISTS idx_prediction_comments_prediction ON prediction_comments(prediction_id)")

	// === Create Agent Follows Table ===
	
	db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_follows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			deleted_at TIMESTAMP,
			follower_id INTEGER NOT NULL,
			followed_id INTEGER NOT NULL,
			FOREIGN KEY (follower_id) REFERENCES agents(id),
			FOREIGN KEY (followed_id) REFERENCES agents(id)
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
					((CAST(correct_predictions AS REAL) / total_predictions * 100) * total_predictions + 50 * 10) / (total_predictions + 10)
				ELSE 50 
			END,
			resolved_predictions = correct_predictions,
			composite_score = accuracy_score * 0.4
		WHERE accuracy_score = 50 OR accuracy_score IS NULL
	`)

	// Set account_balance to 0 for all agents (no longer used)
	db.Exec("UPDATE agents SET account_balance = 0")

	return nil
}
