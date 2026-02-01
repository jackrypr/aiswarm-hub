package verification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
	"socialpredict/middleware"
	"socialpredict/models"
)

// PendingSubmission represents a submission awaiting verification
type PendingSubmission struct {
	gorm.Model
	ID                     int64      `json:"id" gorm:"primary_key"`
	SubmissionType         string     `json:"submissionType" gorm:"not null"` // "market" or "prediction"
	SubmitterAgentID       int64      `json:"submitterAgentId" gorm:"not null"`
	Payload                string     `json:"payload" gorm:"type:text"`
	AutoVerificationStatus string     `json:"autoVerificationStatus" gorm:"default:pending"`
	AutoVerificationResult string     `json:"autoVerificationResult" gorm:"type:text"`
	
	// Council voting
	CouncilStatus    string     `json:"councilStatus" gorm:"default:pending"` // pending, voting, approved, rejected
	VotesFor         int        `json:"votesFor" gorm:"default:0"`
	VotesAgainst     int        `json:"votesAgainst" gorm:"default:0"`
	VotesRequired    int        `json:"votesRequired" gorm:"default:3"`
	ApprovalThreshold float64   `json:"approvalThreshold" gorm:"default:67.0"`
	VotingEndsAt     time.Time  `json:"votingEndsAt"`
	
	FinalStatus      string     `json:"finalStatus"` // approved, rejected, expired
	ResolvedAt       *time.Time `json:"resolvedAt"`
}

// CouncilVote records a validator's vote on a submission
type CouncilVote struct {
	gorm.Model
	ID           int64   `json:"id" gorm:"primary_key"`
	SubmissionID int64   `json:"submissionId" gorm:"not null;index;uniqueIndex:idx_submission_validator"`
	ValidatorID  int64   `json:"validatorId" gorm:"not null;index;uniqueIndex:idx_submission_validator"`
	Vote         string  `json:"vote" gorm:"not null"` // approve or reject
	Reason       string  `json:"reason" gorm:"type:text"`
	Weight       float64 `json:"weight" gorm:"default:1.0"`
}

// ValidatorAgent tracks agents who can vote on submissions
type ValidatorAgent struct {
	AgentID            int64     `json:"agentId" gorm:"primaryKey"`
	IsActive           bool      `json:"isActive" gorm:"default:true"`
	TotalValidations   int64     `json:"totalValidations" gorm:"default:0"`
	CorrectValidations int64     `json:"correctValidations" gorm:"default:0"`
	ValidatorScore     float64   `json:"validatorScore" gorm:"default:50.0"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// MarketPayload is the payload for market submissions
type MarketPayload struct {
	QuestionTitle      string  `json:"questionTitle"`
	Description        string  `json:"description"`
	ResolutionDateTime string  `json:"resolutionDateTime"`
	OutcomeType        string  `json:"outcomeType"`
	InitialProbability float64 `json:"initialProbability"`
}

// VerificationResult contains the auto-verification results
type VerificationResult struct {
	Passed bool                `json:"passed"`
	Checks []VerificationCheck `json:"checks"`
	Errors []string            `json:"errors,omitempty"`
}

// VerificationCheck is a single verification check
type VerificationCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

// SubmitMarketHandler handles POST /v0/submit/market
// All market creation MUST go through this endpoint
func SubmitMarketHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate agent authentication
		agent, httpErr := middleware.ValidateClaimedAgent(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}
		agentID := agent.ID

		var payload MarketPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		// Run FREE auto-verification (no paid APIs)
		result := verifyMarket(payload, db)
		payloadJSON, _ := json.Marshal(payload)
		resultJSON, _ := json.Marshal(result)

		// If basic checks fail, reject immediately (no council needed)
		if !result.Passed {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":      false,
				"status":       "rejected",
				"verification": result,
				"message":      "Auto-verification failed. Please fix the issues and resubmit.",
			})
			return
		}

		// Create submission for council review
		submission := PendingSubmission{
			SubmissionType:         "market",
			SubmitterAgentID:       agentID,
			Payload:                string(payloadJSON),
			AutoVerificationStatus: "passed",
			AutoVerificationResult: string(resultJSON),
			CouncilStatus:          "pending",
			VotesRequired:          3,
			ApprovalThreshold:      67.0,
			VotingEndsAt:           time.Now().Add(24 * time.Hour),
		}

		if err := db.Create(&submission).Error; err != nil {
			http.Error(w, `{"error":"Failed to create submission"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"submissionId": submission.ID,
			"status":       "pending_council_review",
			"verification": result,
			"message":      "Market submitted for council verification. Requires 3+ council votes with 67% approval.",
			"votingEndsAt": submission.VotingEndsAt,
		})
	}
}

// verifyMarket runs FREE verification checks (no paid APIs)
func verifyMarket(payload MarketPayload, db *gorm.DB) VerificationResult {
	var checks []VerificationCheck
	var errors []string

	// Check 1: Resolution date is in the future
	resDate, err := time.Parse(time.RFC3339, payload.ResolutionDateTime)
	futureCheck := VerificationCheck{Name: "future_resolution_date"}
	if err != nil {
		futureCheck.Passed = false
		futureCheck.Reason = "Invalid date format (use RFC3339: 2026-12-31T23:59:59Z)"
	} else if resDate.Before(time.Now()) {
		futureCheck.Passed = false
		futureCheck.Reason = fmt.Sprintf("Resolution date %s is in the past", payload.ResolutionDateTime)
	} else {
		futureCheck.Passed = true
		futureCheck.Reason = "Resolution date is in the future"
	}
	checks = append(checks, futureCheck)

	// Check 2: Question has minimum length
	lengthCheck := VerificationCheck{Name: "question_length"}
	if len(payload.QuestionTitle) < 10 {
		lengthCheck.Passed = false
		lengthCheck.Reason = "Question too short (minimum 10 characters)"
	} else if len(payload.QuestionTitle) > 500 {
		lengthCheck.Passed = false
		lengthCheck.Reason = "Question too long (maximum 500 characters)"
	} else {
		lengthCheck.Passed = true
		lengthCheck.Reason = "Question length OK"
	}
	checks = append(checks, lengthCheck)

	// Check 3: Description has resolution criteria
	criteriaCheck := VerificationCheck{Name: "resolution_criteria"}
	if len(payload.Description) < 20 {
		criteriaCheck.Passed = false
		criteriaCheck.Reason = "Description too short - must include clear resolution criteria"
	} else {
		criteriaCheck.Passed = true
		criteriaCheck.Reason = "Description provided"
	}
	checks = append(checks, criteriaCheck)

	// Check 4: Initial probability is reasonable
	probCheck := VerificationCheck{Name: "initial_probability"}
	if payload.InitialProbability < 0.01 || payload.InitialProbability > 0.99 {
		probCheck.Passed = false
		probCheck.Reason = "Initial probability must be between 1% and 99%"
	} else {
		probCheck.Passed = true
		probCheck.Reason = "Initial probability is reasonable"
	}
	checks = append(checks, probCheck)

	// Check 5: Not about speculative/impossible topics
	specCheck := VerificationCheck{Name: "not_speculative"}
	questionLower := strings.ToLower(payload.QuestionTitle)
	specKeywords := []string{"aliens", "time travel", "magic", "supernatural", "bigfoot", "ufo abduction"}
	isSpeculative := false
	for _, keyword := range specKeywords {
		if strings.Contains(questionLower, keyword) {
			isSpeculative = true
			break
		}
	}
	if isSpeculative {
		specCheck.Passed = false
		specCheck.Reason = "Market appears to be about speculative/unverifiable topics"
	} else {
		specCheck.Passed = true
		specCheck.Reason = "Topic appears verifiable"
	}
	checks = append(checks, specCheck)

	// Check 6: No duplicate markets (simple title matching)
	dupCheck := VerificationCheck{Name: "no_duplicate"}
	var existingCount int64
	searchTerm := payload.QuestionTitle
	if len(searchTerm) > 50 {
		searchTerm = searchTerm[:50]
	}
	db.Model(&models.Market{}).Where("LOWER(question_title) LIKE ?", "%"+strings.ToLower(searchTerm)+"%").Count(&existingCount)
	if existingCount > 0 {
		dupCheck.Passed = false
		dupCheck.Reason = fmt.Sprintf("Similar market may already exist (%d potential matches)", existingCount)
	} else {
		dupCheck.Passed = true
		dupCheck.Reason = "No duplicate markets found"
	}
	checks = append(checks, dupCheck)

	// Determine overall pass/fail
	allPassed := true
	for _, check := range checks {
		if !check.Passed {
			allPassed = false
			errors = append(errors, fmt.Sprintf("%s: %s", check.Name, check.Reason))
		}
	}

	return VerificationResult{
		Passed: allPassed,
		Checks: checks,
		Errors: errors,
	}
}

// VoteOnSubmissionHandler handles POST /v0/council/vote/{submissionId}
func VoteOnSubmissionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate agent authentication
		agent, httpErr := middleware.ValidateClaimedAgent(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}

		// Check if agent is a validator
		var validator ValidatorAgent
		if err := db.Where("agent_id = ? AND is_active = ?", agent.ID, true).First(&validator).Error; err != nil {
			http.Error(w, `{"error":"Agent is not an active council validator"}`, http.StatusForbidden)
			return
		}

		// Get submission ID
		vars := mux.Vars(r)
		submissionID, err := strconv.ParseInt(vars["submissionId"], 10, 64)
		if err != nil {
			http.Error(w, `{"error":"Invalid submission ID"}`, http.StatusBadRequest)
			return
		}

		// Get submission
		var submission PendingSubmission
		if err := db.First(&submission, submissionID).Error; err != nil {
			http.Error(w, `{"error":"Submission not found"}`, http.StatusNotFound)
			return
		}

		// Check submission is still open
		if submission.FinalStatus != "" {
			http.Error(w, `{"error":"Submission is no longer open for voting"}`, http.StatusBadRequest)
			return
		}

		// Check voting hasn't expired
		if time.Now().After(submission.VotingEndsAt) {
			http.Error(w, `{"error":"Voting period has ended"}`, http.StatusBadRequest)
			return
		}

		// Can't vote on own submission
		if submission.SubmitterAgentID == agent.ID {
			http.Error(w, `{"error":"Cannot vote on your own submission"}`, http.StatusForbidden)
			return
		}

		// Check if already voted
		var existingVote CouncilVote
		if err := db.Where("submission_id = ? AND validator_id = ?", submissionID, agent.ID).First(&existingVote).Error; err == nil {
			http.Error(w, `{"error":"Already voted on this submission"}`, http.StatusConflict)
			return
		}

		// Parse vote
		var voteReq struct {
			Vote   string `json:"vote"`   // "approve" or "reject"
			Reason string `json:"reason"` // Optional
		}
		if err := json.NewDecoder(r.Body).Decode(&voteReq); err != nil {
			http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		if voteReq.Vote != "approve" && voteReq.Vote != "reject" {
			http.Error(w, `{"error":"Vote must be 'approve' or 'reject'"}`, http.StatusBadRequest)
			return
		}

		// Calculate vote weight based on validator reputation
		voteWeight := 1.0 + (validator.ValidatorScore / 100.0)

		// Record vote
		vote := CouncilVote{
			SubmissionID: submissionID,
			ValidatorID:  agent.ID,
			Vote:         voteReq.Vote,
			Reason:       voteReq.Reason,
			Weight:       voteWeight,
		}
		if err := db.Create(&vote).Error; err != nil {
			http.Error(w, `{"error":"Failed to record vote"}`, http.StatusInternalServerError)
			return
		}

		// Update submission
		if voteReq.Vote == "approve" {
			submission.VotesFor++
		} else {
			submission.VotesAgainst++
		}
		submission.CouncilStatus = "voting"

		// Update validator stats
		validator.TotalValidations++
		db.Save(&validator)

		// Check if we can resolve
		totalVotes := submission.VotesFor + submission.VotesAgainst
		resolved := false
		var resultMsg string

		if totalVotes >= submission.VotesRequired {
			approvalPct := float64(submission.VotesFor) / float64(totalVotes) * 100
			now := time.Now()
			submission.ResolvedAt = &now

			if approvalPct >= submission.ApprovalThreshold {
				submission.FinalStatus = "approved"
				submission.CouncilStatus = "approved"
				resultMsg = createApprovedMarket(db, &submission)
			} else {
				submission.FinalStatus = "rejected"
				submission.CouncilStatus = "rejected"
				resultMsg = "Submission rejected by council"
			}
			resolved = true
		}

		db.Save(&submission)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"vote":       voteReq.Vote,
			"weight":     voteWeight,
			"votesFor":   submission.VotesFor,
			"votesAgainst": submission.VotesAgainst,
			"resolved":   resolved,
			"result":     resultMsg,
		})
	}
}

// createApprovedMarket creates the actual market after council approval
func createApprovedMarket(db *gorm.DB, submission *PendingSubmission) string {
	var payload MarketPayload
	if err := json.Unmarshal([]byte(submission.Payload), &payload); err != nil {
		return "Failed to parse market payload"
	}

	resDate, _ := time.Parse(time.RFC3339, payload.ResolutionDateTime)

	market := models.Market{
		QuestionTitle:      payload.QuestionTitle,
		Description:        payload.Description,
		OutcomeType:        "BINARY",
		ResolutionDateTime: resDate,
		InitialProbability: payload.InitialProbability,
		CreatorUsername:    fmt.Sprintf("agent_%d", submission.SubmitterAgentID),
	}

	if err := db.Create(&market).Error; err != nil {
		return fmt.Sprintf("Failed to create market: %v", err)
	}

	return fmt.Sprintf("Market created with ID %d", market.ID)
}

// GetCouncilQueueHandler returns submissions awaiting council review
func GetCouncilQueueHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate agent authentication
		agent, httpErr := middleware.ValidateClaimedAgent(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}

		// Check if agent is a validator
		var validator ValidatorAgent
		if err := db.Where("agent_id = ? AND is_active = ?", agent.ID, true).First(&validator).Error; err != nil {
			http.Error(w, `{"error":"Agent is not an active council validator"}`, http.StatusForbidden)
			return
		}

		// Get pending submissions this validator hasn't voted on
		var submissions []PendingSubmission
		subQuery := db.Model(&CouncilVote{}).Select("submission_id").Where("validator_id = ?", agent.ID)

		db.Where("final_status IS NULL OR final_status = ''").
			Where("submitter_agent_id != ?", agent.ID).
			Where("voting_ends_at > ?", time.Now()).
			Where("id NOT IN (?)", subQuery).
			Order("created_at ASC").
			Find(&submissions)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"queue":       submissions,
			"count":       len(submissions),
			"validatorId": agent.ID,
		})
	}
}

// GetValidatorsHandler returns all active validators
func GetValidatorsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var validators []ValidatorAgent
		db.Where("is_active = ?", true).Find(&validators)

		type ValidatorPublic struct {
			AgentID          int64   `json:"agentId"`
			TotalValidations int64   `json:"totalValidations"`
			ValidatorScore   float64 `json:"validatorScore"`
		}

		result := make([]ValidatorPublic, len(validators))
		for i, v := range validators {
			result[i] = ValidatorPublic{
				AgentID:          v.AgentID,
				TotalValidations: v.TotalValidations,
				ValidatorScore:   v.ValidatorScore,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"validators": result,
			"count":      len(result),
		})
	}
}

// RegisterValidatorHandler allows qualified agents to become validators
func RegisterValidatorHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Validate agent authentication
		agent, httpErr := middleware.ValidateClaimedAgent(r, db)
		if httpErr != nil {
			http.Error(w, httpErr.Message, httpErr.StatusCode)
			return
		}

		// Check if already a validator
		var existing ValidatorAgent
		if err := db.Where("agent_id = ?", agent.ID).First(&existing).Error; err == nil {
			if existing.IsActive {
				http.Error(w, `{"error":"Already an active validator"}`, http.StatusConflict)
				return
			}
			// Reactivate
			existing.IsActive = true
			db.Save(&existing)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "Validator reactivated",
				"agentId": agent.ID,
			})
			return
		}

		// Check requirements (relaxed for initial council)
		minPredictions := int64(5)
		if agent.TotalPredictions < minPredictions {
			http.Error(w, fmt.Sprintf(`{"error":"Need at least %d predictions to become validator (have %d)"}`, minPredictions, agent.TotalPredictions), http.StatusForbidden)
			return
		}

		// Create validator
		validator := ValidatorAgent{
			AgentID:        agent.ID,
			IsActive:       true,
			ValidatorScore: 50.0,
		}

		if err := db.Create(&validator).Error; err != nil {
			http.Error(w, `{"error":"Failed to register validator"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Successfully registered as council validator",
			"agentId": agent.ID,
			"note":    "You can now vote on content submissions",
		})
	}
}

// GetPendingSubmissionsHandler returns all pending submissions (public view)
func GetPendingSubmissionsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var submissions []PendingSubmission
		db.Where("final_status IS NULL OR final_status = ''").
			Order("created_at DESC").
			Limit(50).
			Find(&submissions)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":     true,
			"submissions": submissions,
			"count":       len(submissions),
		})
	}
}

// ProcessExpiredSubmissionsHandler processes submissions with expired voting periods
func ProcessExpiredSubmissionsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var submissions []PendingSubmission
		db.Where("(final_status IS NULL OR final_status = '') AND voting_ends_at < ?", time.Now()).Find(&submissions)

		processed := 0
		for _, s := range submissions {
			totalVotes := s.VotesFor + s.VotesAgainst
			now := time.Now()
			s.ResolvedAt = &now

			if totalVotes == 0 {
				s.FinalStatus = "expired"
				s.CouncilStatus = "expired"
			} else {
				approvalPct := float64(s.VotesFor) / float64(totalVotes) * 100
				if approvalPct >= s.ApprovalThreshold {
					s.FinalStatus = "approved"
					s.CouncilStatus = "approved"
					createApprovedMarket(db, &s)
				} else {
					s.FinalStatus = "rejected"
					s.CouncilStatus = "rejected"
				}
			}
			db.Save(&s)
			processed++
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":   true,
			"processed": processed,
		})
	}
}
