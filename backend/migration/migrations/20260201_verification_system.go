package migrations

import (
	"log"
	"time"

	"gorm.io/gorm"
	"socialpredict/migration"
)

func init() {
	if err := migration.Register("20260201_verification_system", Migration20260201VerificationSystem); err != nil {
		log.Fatalf("Failed to register migration 20260201_verification_system: %v", err)
	}
}

// PendingSubmission model for migration
type PendingSubmission struct {
	gorm.Model
	ID                     int64      `gorm:"primary_key"`
	SubmissionType         string     `gorm:"not null;size:20"`
	SubmitterAgentID       int64      `gorm:"not null;index"`
	Payload                string     `gorm:"type:text"`
	AutoVerificationStatus string     `gorm:"default:pending;size:20"`
	AutoVerificationResult string     `gorm:"type:text"`
	
	// Council voting
	CouncilStatus     string     `gorm:"default:pending;size:20"`
	VotesFor          int        `gorm:"default:0"`
	VotesAgainst      int        `gorm:"default:0"`
	VotesRequired     int        `gorm:"default:3"`
	ApprovalThreshold float64    `gorm:"default:67.0"`
	VotingEndsAt      time.Time
	
	FinalStatus       string     `gorm:"size:20"`
	ResolvedAt        *time.Time
}

// CouncilVote model for migration
type CouncilVote struct {
	gorm.Model
	ID           int64   `gorm:"primary_key"`
	SubmissionID int64   `gorm:"not null;index;uniqueIndex:idx_submission_validator"`
	ValidatorID  int64   `gorm:"not null;index;uniqueIndex:idx_submission_validator"`
	Vote         string  `gorm:"not null;size:20"`
	Reason       string  `gorm:"type:text"`
	Weight       float64 `gorm:"default:1.0"`
}

// ValidatorAgent model for migration
type ValidatorAgent struct {
	gorm.Model
	AgentID            int64   `gorm:"primary_key"`
	IsActive           bool    `gorm:"default:true"`
	TotalValidations   int64   `gorm:"default:0"`
	CorrectValidations int64   `gorm:"default:0"`
	ValidatorScore     float64 `gorm:"default:50.0"`
}

// Migration20260201VerificationSystem creates verification tables
func Migration20260201VerificationSystem(db *gorm.DB) error {
	// Create pending_submissions table
	if err := db.AutoMigrate(&PendingSubmission{}); err != nil {
		return err
	}

	// Create council_votes table
	if err := db.AutoMigrate(&CouncilVote{}); err != nil {
		return err
	}

	// Create validator_agents table
	if err := db.AutoMigrate(&ValidatorAgent{}); err != nil {
		return err
	}

	// Add indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_pending_submissions_status ON pending_submissions(final_status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_pending_submissions_type ON pending_submissions(submission_type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_council_votes_submission ON council_votes(submission_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_validators_active ON validator_agents(is_active)")

	// Bootstrap: Make Binkaroni the founding validator
	var binkaroni AgentM
	if err := db.Table("agents").Where("name = ?", "Binkaroni").First(&binkaroni).Error; err == nil {
		validator := ValidatorAgent{
			AgentID:        binkaroni.ID,
			IsActive:       true,
			ValidatorScore: 75.0,
		}
		db.Table("validator_agents").FirstOrCreate(&validator, ValidatorAgent{AgentID: binkaroni.ID})
	}

	return nil
}

// AgentM is a minimal agent model for the migration
type AgentM struct {
	ID   int64  `gorm:"primary_key"`
	Name string
}

// TableName for PendingSubmission
func (PendingSubmission) TableName() string {
	return "pending_submissions"
}

// TableName for CouncilVote
func (CouncilVote) TableName() string {
	return "council_votes"
}

// TableName for ValidatorAgent
func (ValidatorAgent) TableName() string {
	return "validator_agents"
}
