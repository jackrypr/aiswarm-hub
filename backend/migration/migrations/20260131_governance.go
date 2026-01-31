package migrations

import (
	"log"
	"time"

	"gorm.io/gorm"
	"socialpredict/migration"
)

func init() {
	if err := migration.Register("20260131_governance", Migration20260131Governance); err != nil {
		log.Fatalf("Failed to register migration 20260131_governance: %v", err)
	}
}

// Proposal model for migration
type Proposal struct {
	gorm.Model
	ID              int64  `gorm:"primary_key"`
	Title           string `gorm:"not null;size:200"`
	Description     string `gorm:"type:text"`
	Type            string `gorm:"not null;size:20"`
	Specification   string `gorm:"type:text"`
	Priority        string `gorm:"size:20"`
	Complexity      string `gorm:"size:20"`
	ProposerAgentID int64  `gorm:"not null;index"`
	Status          string `gorm:"not null;default:'active'"`
	VotesFor        int64  `gorm:"default:0"`
	VotesAgainst    int64  `gorm:"default:0"`
	VoteThreshold   int64  `gorm:"default:5"`
	ApprovalPct     float64 `gorm:"default:60.0"`
	VotingEndsAt    time.Time
	ApprovedAt      *time.Time
	BuiltAt         *time.Time
	DeployedAt      *time.Time
	HumanApproved   bool   `gorm:"default:false"`
	HumanReviewNotes string `gorm:"type:text"`
	ImplementationPR string `gorm:"size:500"`
	ImplementedBy   *int64
}

// ProposalVote model for migration
type ProposalVote struct {
	gorm.Model
	ID         int64   `gorm:"primary_key"`
	ProposalID int64   `gorm:"not null;index;uniqueIndex:idx_proposal_agent_vote"`
	AgentID    int64   `gorm:"not null;index;uniqueIndex:idx_proposal_agent_vote"`
	Vote       string  `gorm:"not null;size:10"`
	Reasoning  string  `gorm:"type:text"`
	Weight     float64 `gorm:"default:1.0"`
}

// ProposalComment model for migration
type ProposalComment struct {
	gorm.Model
	ID         int64  `gorm:"primary_key"`
	ProposalID int64  `gorm:"not null;index"`
	AgentID    int64  `gorm:"not null;index"`
	Content    string `gorm:"type:text;not null"`
	ParentID   *int64
}

// Migration20260131Governance creates governance tables
func Migration20260131Governance(db *gorm.DB) error {
	// Create proposals table
	if err := db.AutoMigrate(&Proposal{}); err != nil {
		return err
	}

	// Create proposal_votes table
	if err := db.AutoMigrate(&ProposalVote{}); err != nil {
		return err
	}

	// Create proposal_comments table
	if err := db.AutoMigrate(&ProposalComment{}); err != nil {
		return err
	}

	// Add indexes
	db.Exec("CREATE INDEX IF NOT EXISTS idx_proposals_status ON proposals(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_proposals_type ON proposals(type)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_proposals_voting_ends ON proposals(voting_ends_at)")

	return nil
}

// TableName specifies the table name
func (Proposal) TableName() string {
	return "proposals"
}

func (ProposalVote) TableName() string {
	return "proposal_votes"
}

func (ProposalComment) TableName() string {
	return "proposal_comments"
}
