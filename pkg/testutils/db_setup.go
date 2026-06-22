package testutils

import (
	"fmt"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDB initializes the database connection and migrates models for testing purposes.
func SetupTestDB() *gorm.DB {
	// IMPORTANT: Adjust the user and password to match your local PostgreSQL credentials.
	dsn := "host=localhost user=postgres password=azmi123 dbname=money_saver_test port=5432 sslmode=disable TimeZone=Asia/Jakarta"

	// Set the logger to silent mode to prevent SQL queries from cluttering the test output.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("Failed to connect to the test database: " + err.Error())
	}

	// Auto-migrate all required models to the test database.
	err = db.AutoMigrate(
		&models.User{},
		&models.Workspace{},
		&models.Category{},
		&models.Transaction{},
		&models.TransactionItem{},
		&models.Target{},
		&models.Debt{},
		&models.PendingTransaction{},
		&models.EmailParsed{},
		&models.WorkspaceMember{},
		&models.WorkspaceInvitation{},
		&models.OTP{},
		&models.RefreshToken{},
		&models.RevokeToken{},
	)
	if err != nil {
		panic("Failed to perform database migration for testing: " + err.Error())
	}

	return db
}

// CleanTestDB truncates all tables to reset the database state after each test scenario.
func CleanTestDB(db *gorm.DB) {
	// Truncate all tables with CASCADE to ensure all relational data is completely removed.
	tables := []string{
		"transaction_items", "transactions", "pending_transactions",
		"targets", "categories", "debts", "workspace_members",
		"workspace_invitations", "otps", "refresh_tokens",
		"revoke_tokens", "workspaces", "users", "email_parseds",
	}

	for _, table := range tables {
		db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", table))
	}
}
