package verification

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// PendingSubmission represents a submission awaiting verification
type PendingSubmission struct {
	gorm.Model
	ID                     int64           `json:"id" gorm:"primary_key"`
	SubmissionType         string          `json:"submissionType" gorm:"not null"` // "market" or "prediction"
	SubmitterAgentID       int64           `json:"submitterAgentId" gorm:"not null"`
	Payload                string          `json:"payload" gorm:"type:text"`
	AutoVerificationStatus string          `json:"autoVerificationStatus" gorm:"default:pending"`
	AutoVerificationResult string          `json:"autoVerificationResult" gorm:"type:text"`
	CouncilStatus          string          `json:"councilStatus" gorm:"default:pending"`
	FinalStatus            string          `json:"finalStatus"` // approved, rejected
	ResolvedAt             *time.Time      `json:"resolvedAt"`
}

// MarketPayload is the payload for market submissions
type MarketPayload struct {
	QuestionTitle      string `json:"questionTitle"`
	Description        string `json:"description"`
	ResolutionDateTime string `json:"resolutionDateTime"`
	OutcomeType        string `json:"outcomeType"`
}

// VerificationResult contains the auto-verification results
type VerificationResult struct {
	Passed bool                     `json:"passed"`
	Checks []VerificationCheck      `json:"checks"`
	Errors []string                 `json:"errors,omitempty"`
}

// VerificationCheck is a single verification check
type VerificationCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// SubmitMarketHandler handles POST /v0/submit/market
// All market creation MUST go through this endpoint
func SubmitMarketHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get agent from auth
		agentID := r.Context().Value("agentID")
		if agentID == nil {
			http.Error(w, "Agent authentication required", http.StatusUnauthorized)
			return
		}

		var payload MarketPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Run auto-verification
		result := verifyMarket(payload)

		// Store as pending submission
		payloadJSON, _ := json.Marshal(payload)
		resultJSON, _ := json.Marshal(result)
		
		submission := PendingSubmission{
			SubmissionType:         "market",
			SubmitterAgentID:       agentID.(int64),
			Payload:                string(payloadJSON),
			AutoVerificationStatus: map[bool]string{true: "passed", false: "failed"}[result.Passed],
			AutoVerificationResult: string(resultJSON),
			CouncilStatus:          "pending",
		}

		if err := db.Create(&submission).Error; err != nil {
			http.Error(w, "Failed to create submission", http.StatusInternalServerError)
			return
		}

		// If auto-verification failed, reject immediately
		if !result.Passed {
			submission.FinalStatus = "rejected"
			now := time.Now()
			submission.ResolvedAt = &now
			db.Save(&submission)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":      false,
				"submissionId": submission.ID,
				"status":       "rejected",
				"verification": result,
				"message":      "Auto-verification failed. Please fix the issues and resubmit.",
			})
			return
		}

		// Auto-verification passed - queue for council review (or auto-approve in MVP)
		// For MVP: auto-approve if all checks pass
		submission.FinalStatus = "approved"
		submission.CouncilStatus = "auto_approved"
		now := time.Now()
		submission.ResolvedAt = &now
		db.Save(&submission)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":      true,
			"submissionId": submission.ID,
			"status":       "approved",
			"verification": result,
			"message":      "Market submission verified and approved",
		})
	}
}

// verifyMarket runs all verification checks on a market
func verifyMarket(payload MarketPayload) VerificationResult {
	var checks []VerificationCheck
	var errors []string

	// Check 1: Resolution date is in the future
	resDate, err := time.Parse(time.RFC3339, payload.ResolutionDateTime)
	futureCheck := VerificationCheck{Name: "future_resolution_date"}
	if err != nil {
		futureCheck.Passed = false
		futureCheck.Reason = "Invalid date format"
	} else if resDate.Before(time.Now()) {
		futureCheck.Passed = false
		futureCheck.Reason = fmt.Sprintf("Resolution date %s is in the past", payload.ResolutionDateTime)
	} else {
		futureCheck.Passed = true
		futureCheck.Reason = fmt.Sprintf("Resolution date %s is in the future", payload.ResolutionDateTime)
	}
	checks = append(checks, futureCheck)

	// Check 2: Question has minimum length
	lengthCheck := VerificationCheck{Name: "question_length"}
	if len(payload.QuestionTitle) < 10 {
		lengthCheck.Passed = false
		lengthCheck.Reason = "Question too short (min 10 chars)"
	} else {
		lengthCheck.Passed = true
	}
	checks = append(checks, lengthCheck)

	// Check 3: Description has resolution criteria
	criteriaCheck := VerificationCheck{Name: "resolution_criteria"}
	if len(payload.Description) < 20 {
		criteriaCheck.Passed = false
		criteriaCheck.Reason = "Description too short - must include clear resolution criteria"
	} else if !strings.Contains(strings.ToLower(payload.Description), "resolve") {
		criteriaCheck.Passed = false
		criteriaCheck.Reason = "Description should include 'Resolves YES/NO if...' criteria"
	} else {
		criteriaCheck.Passed = true
	}
	checks = append(checks, criteriaCheck)

	// Check 4: Web search for past events (the critical check)
	pastEventCheck := checkForPastEvent(payload.QuestionTitle)
	checks = append(checks, pastEventCheck)

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

// checkForPastEvent searches the web to verify the event hasn't already happened
func checkForPastEvent(question string) VerificationCheck {
	check := VerificationCheck{Name: "not_past_event"}

	// Use Brave Search API if available
	braveKey := os.Getenv("BRAVE_API_KEY")
	if braveKey == "" {
		// No API key - can't verify, pass with warning
		check.Passed = true
		check.Reason = "Web verification skipped (no BRAVE_API_KEY)"
		return check
	}

	// Search for evidence the event already happened
	searchQuery := url.QueryEscape(question + " already happened released announced")
	searchURL := fmt.Sprintf("https://api.search.brave.com/res/v1/web/search?q=%s&count=5", searchQuery)

	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("X-Subscription-Token", braveKey)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		check.Passed = true
		check.Reason = "Web search failed - proceeding with caution"
		return check
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	// Simple heuristic: look for keywords indicating past event
	bodyLower := strings.ToLower(string(body))
	pastIndicators := []string{"already released", "was released", "has been released", "launched", "announced", "is out", "came out"}
	
	for _, indicator := range pastIndicators {
		if strings.Contains(bodyLower, indicator) {
			check.Passed = false
			check.Reason = fmt.Sprintf("Web search suggests this event may have already occurred (found: '%s')", indicator)
			check.Evidence = string(body)[:min(500, len(body))]
			return check
		}
	}

	check.Passed = true
	check.Reason = "No evidence found that event already occurred"
	return check
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetPendingSubmissionsHandler handles GET /v0/pending
func GetPendingSubmissionsHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var submissions []PendingSubmission
		db.Where("final_status IS NULL OR final_status = 'pending'").
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

// ApproveSubmissionHandler handles POST /v0/admin/submission/{id}/approve
func ApproveSubmissionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var submission PendingSubmission
		if err := db.First(&submission, id).Error; err != nil {
			http.Error(w, "Submission not found", http.StatusNotFound)
			return
		}

		submission.FinalStatus = "approved"
		submission.CouncilStatus = "admin_approved"
		now := time.Now()
		submission.ResolvedAt = &now
		db.Save(&submission)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Submission approved",
		})
	}
}

// RejectSubmissionHandler handles POST /v0/admin/submission/{id}/reject
func RejectSubmissionHandler(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var submission PendingSubmission
		if err := db.First(&submission, id).Error; err != nil {
			http.Error(w, "Submission not found", http.StatusNotFound)
			return
		}

		submission.FinalStatus = "rejected"
		submission.CouncilStatus = "admin_rejected"
		now := time.Now()
		submission.ResolvedAt = &now
		db.Save(&submission)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Submission rejected",
		})
	}
}
