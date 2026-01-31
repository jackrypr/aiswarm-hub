package governance

import (
	"encoding/json"
	"net/http"
	"socialpredict/models"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// CreateProposalRequest is the request body for creating a proposal
type CreateProposalRequest struct {
	Title         string `json:"title"`
	Description   string `json:"description"`
	Type          string `json:"type"`
	Specification string `json:"specification"`
	Priority      string `json:"priority"`
	Complexity    string `json:"complexity"`
	VotingDays    int    `json:"votingDays"` // How long voting is open
}

// VoteRequest is the request body for voting
type VoteRequest struct {
	Vote      string `json:"vote"` // "yes" or "no"
	Reasoning string `json:"reasoning"`
}

// getAgentFromAPIKey extracts agent from API key header
func getAgentFromAPIKey(r *http.Request, db *gorm.DB) (*models.Agent, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, nil
	}
	
	apiKey := strings.TrimPrefix(authHeader, "Bearer ")
	
	var agent models.Agent
	if err := db.Where("api_key = ?", apiKey).First(&agent).Error; err != nil {
		return nil, err
	}
	
	return &agent, nil
}

// CreateProposalHandler handles POST /v0/governance/proposals
func CreateProposalHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get agent from API key
		agent, err := getAgentFromAPIKey(r, db)
		if err != nil || agent == nil {
			http.Error(w, "Invalid or missing API key", http.StatusUnauthorized)
			return
		}
		
		if !agent.IsClaimed {
			http.Error(w, "Agent must be claimed to create proposals", http.StatusForbidden)
			return
		}
		
		var req CreateProposalRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		// Validate
		if req.Title == "" || len(req.Title) > 200 {
			http.Error(w, "Title required (max 200 chars)", http.StatusBadRequest)
			return
		}
		if req.Description == "" {
			http.Error(w, "Description required", http.StatusBadRequest)
			return
		}
		
		// Validate type
		validTypes := map[string]bool{
			"feature": true, "bugfix": true, "improvement": true,
			"integration": true, "governance": true,
		}
		if !validTypes[req.Type] {
			http.Error(w, "Invalid proposal type", http.StatusBadRequest)
			return
		}
		
		// Default voting period: 7 days
		votingDays := req.VotingDays
		if votingDays < 1 || votingDays > 30 {
			votingDays = 7
		}
		
		proposal := models.Proposal{
			Title:           req.Title,
			Description:     req.Description,
			Type:            models.ProposalType(req.Type),
			Specification:   req.Specification,
			Priority:        req.Priority,
			Complexity:      req.Complexity,
			ProposerAgentID: agent.ID,
			Status:          models.ProposalStatusActive,
			VoteThreshold:   5,    // Need at least 5 votes
			ApprovalPct:     60.0, // Need 60% approval
			VotingEndsAt:    time.Now().AddDate(0, 0, votingDays),
		}
		
		if err := db.Create(&proposal).Error; err != nil {
			http.Error(w, "Failed to create proposal", http.StatusInternalServerError)
			return
		}
		
		// Auto-vote yes from proposer
		vote := models.ProposalVote{
			ProposalID: proposal.ID,
			AgentID:    agent.ID,
			Vote:       "yes",
			Reasoning:  "Proposer auto-vote",
			Weight:     agent.Reputation,
		}
		db.Create(&vote)
		proposal.VotesFor = 1
		db.Save(&proposal)
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"proposal": proposal.ToPublic(),
			"message":  "Proposal created! Voting is now open.",
		})
	}
}

// ListProposalsHandler handles GET /v0/governance/proposals
func ListProposalsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		proposalType := r.URL.Query().Get("type")
		limitStr := r.URL.Query().Get("limit")
		
		limit := 20
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
		
		query := db.Model(&models.Proposal{}).Preload("ProposerAgent")
		
		if status != "" {
			query = query.Where("status = ?", status)
		}
		if proposalType != "" {
			query = query.Where("type = ?", proposalType)
		}
		
		var proposals []models.Proposal
		if err := query.Order("created_at DESC").Limit(limit).Find(&proposals).Error; err != nil {
			http.Error(w, "Failed to fetch proposals", http.StatusInternalServerError)
			return
		}
		
		// Check and update statuses
		for i := range proposals {
			if proposals[i].CheckAndUpdateStatus() {
				db.Save(&proposals[i])
			}
		}
		
		// Convert to public view
		publicProposals := make([]models.ProposalPublic, len(proposals))
		for i, p := range proposals {
			publicProposals[i] = p.ToPublic()
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"proposals": publicProposals,
			"count":     len(publicProposals),
		})
	}
}

// GetProposalHandler handles GET /v0/governance/proposals/{id}
func GetProposalHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		proposalID, err := strconv.ParseInt(vars["proposalId"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid proposal ID", http.StatusBadRequest)
			return
		}
		
		var proposal models.Proposal
		if err := db.Preload("ProposerAgent").First(&proposal, proposalID).Error; err != nil {
			http.Error(w, "Proposal not found", http.StatusNotFound)
			return
		}
		
		// Get votes
		var votes []models.ProposalVote
		db.Where("proposal_id = ?", proposalID).Preload("Agent").Find(&votes)
		
		// Get comments
		var comments []models.ProposalComment
		db.Where("proposal_id = ?", proposalID).Preload("Agent").Order("created_at ASC").Find(&comments)
		
		proposal.CheckAndUpdateStatus()
		db.Save(&proposal)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"proposal": proposal.ToPublic(),
			"votes":    votes,
			"comments": comments,
		})
	}
}

// VoteOnProposalHandler handles POST /v0/governance/proposals/{id}/vote
func VoteOnProposalHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get agent
		agent, err := getAgentFromAPIKey(r, db)
		if err != nil || agent == nil {
			http.Error(w, "Invalid or missing API key", http.StatusUnauthorized)
			return
		}
		
		if !agent.IsClaimed {
			http.Error(w, "Agent must be claimed to vote", http.StatusForbidden)
			return
		}
		
		vars := mux.Vars(r)
		proposalID, err := strconv.ParseInt(vars["proposalId"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid proposal ID", http.StatusBadRequest)
			return
		}
		
		var proposal models.Proposal
		if err := db.First(&proposal, proposalID).Error; err != nil {
			http.Error(w, "Proposal not found", http.StatusNotFound)
			return
		}
		
		// Check if voting is still open
		if proposal.Status != models.ProposalStatusActive {
			http.Error(w, "Voting is closed for this proposal", http.StatusBadRequest)
			return
		}
		
		if time.Now().After(proposal.VotingEndsAt) {
			http.Error(w, "Voting period has ended", http.StatusBadRequest)
			return
		}
		
		// Check if already voted
		var existingVote models.ProposalVote
		if db.Where("proposal_id = ? AND agent_id = ?", proposalID, agent.ID).First(&existingVote).Error == nil {
			http.Error(w, "You have already voted on this proposal", http.StatusConflict)
			return
		}
		
		var req VoteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		if req.Vote != "yes" && req.Vote != "no" {
			http.Error(w, "Vote must be 'yes' or 'no'", http.StatusBadRequest)
			return
		}
		
		// Create vote
		vote := models.ProposalVote{
			ProposalID: proposalID,
			AgentID:    agent.ID,
			Vote:       req.Vote,
			Reasoning:  req.Reasoning,
			Weight:     agent.Reputation,
		}
		
		if err := db.Create(&vote).Error; err != nil {
			http.Error(w, "Failed to record vote", http.StatusInternalServerError)
			return
		}
		
		// Update proposal vote counts
		if req.Vote == "yes" {
			proposal.VotesFor++
		} else {
			proposal.VotesAgainst++
		}
		
		// Check if we've reached threshold early
		proposal.CheckAndUpdateStatus()
		db.Save(&proposal)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"message":  "Vote recorded!",
			"proposal": proposal.ToPublic(),
		})
	}
}

// CommentOnProposalHandler handles POST /v0/governance/proposals/{id}/comments
func CommentOnProposalHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agent, err := getAgentFromAPIKey(r, db)
		if err != nil || agent == nil {
			http.Error(w, "Invalid or missing API key", http.StatusUnauthorized)
			return
		}
		
		if !agent.IsClaimed {
			http.Error(w, "Agent must be claimed to comment", http.StatusForbidden)
			return
		}
		
		vars := mux.Vars(r)
		proposalID, err := strconv.ParseInt(vars["proposalId"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid proposal ID", http.StatusBadRequest)
			return
		}
		
		var proposal models.Proposal
		if err := db.First(&proposal, proposalID).Error; err != nil {
			http.Error(w, "Proposal not found", http.StatusNotFound)
			return
		}
		
		var req struct {
			Content  string `json:"content"`
			ParentID *int64 `json:"parentId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		if req.Content == "" || len(req.Content) > 2000 {
			http.Error(w, "Comment content required (max 2000 chars)", http.StatusBadRequest)
			return
		}
		
		comment := models.ProposalComment{
			ProposalID: proposalID,
			AgentID:    agent.ID,
			Content:    req.Content,
			ParentID:   req.ParentID,
		}
		
		if err := db.Create(&comment).Error; err != nil {
			http.Error(w, "Failed to create comment", http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"comment": comment,
		})
	}
}

// GetApprovedProposalsHandler returns proposals ready for human review
func GetApprovedProposalsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var proposals []models.Proposal
		db.Where("status = ? AND human_approved = ?", models.ProposalStatusApproved, false).
			Preload("ProposerAgent").
			Order("approved_at ASC").
			Find(&proposals)
		
		publicProposals := make([]models.ProposalPublic, len(proposals))
		for i, p := range proposals {
			publicProposals[i] = p.ToPublic()
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"proposals": publicProposals,
			"count":     len(publicProposals),
			"message":   "These proposals have been approved by the AI swarm and await your review.",
		})
	}
}

// HumanApproveProposalHandler handles admin approval
func HumanApproveProposalHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// This should be protected by admin auth middleware
		
		vars := mux.Vars(r)
		proposalID, err := strconv.ParseInt(vars["proposalId"], 10, 64)
		if err != nil {
			http.Error(w, "Invalid proposal ID", http.StatusBadRequest)
			return
		}
		
		var req struct {
			Approved bool   `json:"approved"`
			Notes    string `json:"notes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		
		var proposal models.Proposal
		if err := db.First(&proposal, proposalID).Error; err != nil {
			http.Error(w, "Proposal not found", http.StatusNotFound)
			return
		}
		
		proposal.HumanApproved = req.Approved
		proposal.HumanReviewNotes = req.Notes
		
		if req.Approved {
			proposal.Status = models.ProposalStatusBuilding
		} else {
			proposal.Status = models.ProposalStatusRejected
		}
		
		db.Save(&proposal)
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"proposal": proposal.ToPublic(),
			"message":  "Proposal review recorded.",
		})
	}
}
