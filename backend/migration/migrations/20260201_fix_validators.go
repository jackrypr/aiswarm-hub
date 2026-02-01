package migrations

import (
	"log"
	"time"

	"gorm.io/gorm"
	"socialpredict/migration"
)

func init() {
	if err := migration.Register("20260201_fix_validators", Migration20260201FixValidators); err != nil {
		log.Fatalf("Failed to register migration 20260201_fix_validators: %v", err)
	}
}

// FixedValidatorAgent with correct schema
type FixedValidatorAgent struct {
	AgentID            int64     `gorm:"primaryKey;column:agent_id"`
	IsActive           bool      `gorm:"default:true;column:is_active"`
	TotalValidations   int64     `gorm:"default:0;column:total_validations"`
	CorrectValidations int64     `gorm:"default:0;column:correct_validations"`
	ValidatorScore     float64   `gorm:"default:50.0;column:validator_score"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (FixedValidatorAgent) TableName() string {
	return "validator_agents"
}

// Migration20260201FixValidators fixes the validator_agents table schema
func Migration20260201FixValidators(db *gorm.DB) error {
	// Drop the old table if it exists (fresh start)
	db.Exec("DROP TABLE IF EXISTS validator_agents")
	
	// Create with correct schema
	if err := db.AutoMigrate(&FixedValidatorAgent{}); err != nil {
		return err
	}
	
	// Bootstrap: Make Binkaroni the founding validator
	db.Exec(`INSERT INTO validator_agents (agent_id, is_active, total_validations, correct_validations, validator_score, created_at, updated_at) 
		SELECT 3, true, 0, 0, 75.0, NOW(), NOW() 
		WHERE EXISTS (SELECT 1 FROM agents WHERE id = 3)
		ON CONFLICT (agent_id) DO NOTHING`)
	
	return nil
}
