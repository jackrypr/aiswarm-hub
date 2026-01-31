package models

import (
	"time"

	"gorm.io/gorm"
)

// ProposalStatus represents the lifecycle of a proposal
type ProposalStatus string

const (
	ProposalStatusDraft     ProposalStatus = "draft"
	ProposalStatusActive    ProposalStatus = "active"
	ProposalStatusApproved  ProposalStatus = "approved"
	ProposalStatusRejected  ProposalStatus = "rejected"
	ProposalStatusBuilding  ProposalStatus = "building"
	ProposalStatusDeployed  ProposalStatus = "deployed"
)

// ProposalType categorizes what kind of change is being proposed
type ProposalType string

const (
	ProposalTypeFeature     ProposalType = "feature"
	ProposalTypeBugfix      ProposalType = "bugfix"
	ProposalTypeImprovement ProposalType = "improvement"
	ProposalTypeIntegration ProposalType = "integration"
	ProposalTypeGovernance  ProposalType = "governance"
)

// Proposal represents a feature/change proposed by an AI agent
type Proposal struct {
	gorm.Model
	ID          int64          `json:"id" gorm:"primary_key"`
	
	// Content
	Title       string         `json:"title" gorm:"not null;size:200"`
	Description string         `json:"description" gorm:"type:text"`
	Type        ProposalType   `json:"type" gorm:"not null;size:20"`
	
	// Technical Details (for implementation)
	Specification string       `json:"specification" gorm:"type:text"` // Detailed technical spec
	Priority      string       `json:"priority" gorm:"size:20"`        // low, medium, high, critical
	Complexity    string       `json:"complexity" gorm:"size:20"`      // simple, moderate, complex
	
	// Proposer
	ProposerAgentID int64      `json:"proposerAgentId" gorm:"not null;index"`
	ProposerAgent   Agent      `json:"proposerAgent" gorm:"foreignKey:ProposerAgentID"`
	
	// Voting
	Status        ProposalStatus `json:"status" gorm:"not null;default:'active'"`
	VotesFor      int64          `json:"votesFor" gorm:"default:0"`
	VotesAgainst  int64          `json:"votesAgainst" gorm:"default:0"`
	VoteThreshold int64          `json:"voteThreshold" gorm:"default:5"`    // Min votes needed
	ApprovalPct   float64        `json:"approvalPct" gorm:"default:60.0"`   // % needed to pass
	
	// Timeline
	VotingEndsAt  time.Time      `json:"votingEndsAt"`
	ApprovedAt    *time.Time     `json:"approvedAt,omitempty"`
	BuiltAt       *time.Time     `json:"builtAt,omitempty"`
	DeployedAt    *time.Time     `json:"deployedAt,omitempty"`
	
	// Human Review
	HumanApproved    bool       `json:"humanApproved" gorm:"default:false"`
	HumanReviewNotes string     `json:"humanReviewNotes" gorm:"type:text"`
	
	// Implementation
	ImplementationPR string     `json:"implementationPr" gorm:"size:500"` // GitHub PR link
	ImplementedBy    *int64     `json:"implementedBy,omitempty"`          // Agent who built it
}

// ProposalVote records an agent's vote on a proposal
type ProposalVote struct {
	gorm.Model
	ID         int64  `json:"id" gorm:"primary_key"`
	ProposalID int64  `json:"proposalId" gorm:"not null;index;uniqueIndex:idx_proposal_agent"`
	AgentID    int64  `json:"agentId" gorm:"not null;index;uniqueIndex:idx_proposal_agent"`
	
	Vote       string `json:"vote" gorm:"not null;size:10"` // "yes" or "no"
	Reasoning  string `json:"reasoning" gorm:"type:text"`
	Weight     float64 `json:"weight" gorm:"default:1.0"`   // Based on agent reputation
	
	Agent      Agent  `json:"agent" gorm:"foreignKey:AgentID"`
}

// ProposalComment for discussion threads
type ProposalComment struct {
	gorm.Model
	ID         int64  `json:"id" gorm:"primary_key"`
	ProposalID int64  `json:"proposalId" gorm:"not null;index"`
	AgentID    int64  `json:"agentId" gorm:"not null;index"`
	
	Content    string `json:"content" gorm:"type:text;not null"`
	ParentID   *int64 `json:"parentId,omitempty"` // For threaded comments
	
	Agent      Agent  `json:"agent" gorm:"foreignKey:AgentID"`
}

// ProposalPublic is the public view of a proposal
type ProposalPublic struct {
	ID              int64          `json:"id"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	Type            ProposalType   `json:"type"`
	Specification   string         `json:"specification"`
	Priority        string         `json:"priority"`
	Complexity      string         `json:"complexity"`
	ProposerAgentID int64          `json:"proposerAgentId"`
	ProposerName    string         `json:"proposerName"`
	Status          ProposalStatus `json:"status"`
	VotesFor        int64          `json:"votesFor"`
	VotesAgainst    int64          `json:"votesAgainst"`
	VoteThreshold   int64          `json:"voteThreshold"`
	ApprovalPct     float64        `json:"approvalPct"`
	CurrentPct      float64        `json:"currentPct"` // Calculated
	VotingEndsAt    time.Time      `json:"votingEndsAt"`
	HumanApproved   bool           `json:"humanApproved"`
	CreatedAt       time.Time      `json:"createdAt"`
}

// ToPublic converts Proposal to ProposalPublic
func (p *Proposal) ToPublic() ProposalPublic {
	totalVotes := p.VotesFor + p.VotesAgainst
	currentPct := 0.0
	if totalVotes > 0 {
		currentPct = float64(p.VotesFor) / float64(totalVotes) * 100
	}
	
	proposerName := ""
	if p.ProposerAgent.Name != "" {
		proposerName = p.ProposerAgent.Name
	}
	
	return ProposalPublic{
		ID:              p.ID,
		Title:           p.Title,
		Description:     p.Description,
		Type:            p.Type,
		Specification:   p.Specification,
		Priority:        p.Priority,
		Complexity:      p.Complexity,
		ProposerAgentID: p.ProposerAgentID,
		ProposerName:    proposerName,
		Status:          p.Status,
		VotesFor:        p.VotesFor,
		VotesAgainst:    p.VotesAgainst,
		VoteThreshold:   p.VoteThreshold,
		ApprovalPct:     p.ApprovalPct,
		CurrentPct:      currentPct,
		VotingEndsAt:    p.VotingEndsAt,
		HumanApproved:   p.HumanApproved,
		CreatedAt:       p.CreatedAt,
	}
}

// CheckAndUpdateStatus checks if voting is complete and updates status
func (p *Proposal) CheckAndUpdateStatus() bool {
	// Check if voting period ended
	if time.Now().After(p.VotingEndsAt) && p.Status == ProposalStatusActive {
		totalVotes := p.VotesFor + p.VotesAgainst
		
		// Need minimum votes
		if totalVotes < p.VoteThreshold {
			p.Status = ProposalStatusRejected
			return true
		}
		
		// Check approval percentage
		approvalPct := float64(p.VotesFor) / float64(totalVotes) * 100
		if approvalPct >= p.ApprovalPct {
			p.Status = ProposalStatusApproved
			now := time.Now()
			p.ApprovedAt = &now
		} else {
			p.Status = ProposalStatusRejected
		}
		return true
	}
	return false
}
