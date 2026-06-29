package testutils

import (
	"fmt"
	"os"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func SetupTestDB() *gorm.DB {
	// Ambil dari environment, kalau kosong (lagi di laptop), pake default lokal
	host := os.Getenv("TEST_DB_HOST")
	if host == "" {
		host = "localhost"
	}

	user := os.Getenv("TEST_DB_USER")
	if user == "" {
		user = "postgres"
	}

	password := os.Getenv("TEST_DB_PASSWORD")
	if password == "" {
		password = "azmi123"
	}

	dbName := os.Getenv("TEST_DB_NAME")
	if dbName == "" {
		dbName = "money_saver_test"
	}

	port := os.Getenv("TEST_DB_PORT")
	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		host, user, password, dbName, port)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		panic("Failed to connect to the test database: " + err.Error())
	}

	err = db.AutoMigrate(
		&models.User{}, &models.Workspace{}, &models.Category{},
		&models.Transaction{}, &models.TransactionItem{}, &models.Target{},
		&models.Debt{}, &models.PendingTransaction{}, &models.EmailParsed{},
		&models.WorkspaceMember{}, &models.WorkspaceInvitation{}, &models.OTP{},
		&models.RefreshToken{}, &models.RevokeToken{},
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
