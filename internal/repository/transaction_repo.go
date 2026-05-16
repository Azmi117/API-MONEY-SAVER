package repository

import (
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransactionRepository interface {
	Create(transaction *models.Transaction) error
	IsDuplicate(workspaceID uint, amount float64, merchant string, date time.Time) (bool, error)
	UpdateStatus(id uint, status string) error
	GetByWorkspaceID(workspaceID uint) ([]models.Transaction, error)
	Delete(id uint) error
	GetByGmailID(gmailID string) (*models.Transaction, error)
	HardDelete(id uint) error
	CreateEmailLog(emailLog *models.EmailParsed) error
	GetEmailLogByGmailID(gmailID string) (*models.EmailParsed, error)
	UpdateEmailLogStatus(id uint, status string) error
	GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error)
	GetEmailLogByID(id uint) (*models.EmailParsed, error)
	GetTotalAmountByType(workspaceID uint, txType string, period string) (float64, error)
	FindByID(tx *models.Transaction, id uint) error
	GetTotalByMonth(workspaceID uint, period string) (float64, error)
	GetTotalSavings(workspaceID uint, period string) (float64, error)
	GetEmailParsedByID(id uint) (*models.EmailParsed, error)
	DeleteEmailParsed(id uint) error
	GetSummaryByWorkspace(workspaceID uint, txType string, month string) ([]dto.UserTransactionSummary, error)
	GetTotalByWorkspace(workspaceID uint, txType string, month string) (float64, error)
	CreateWithItems(transaction *models.Transaction) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db}
}

func (r *transactionRepository) FindByID(tx *models.Transaction, id uint) error {
	// Eager load transaction items
	return r.db.Preload("TransactionItems").First(tx, id).Error
}

func (r *transactionRepository) Create(transaction *models.Transaction) error {
	return r.db.Create(transaction).Error
}

func (r *transactionRepository) CreateWithItems(transaction *models.Transaction) error {
	return r.db.Session(&gorm.Session{FullSaveAssociations: true}).Create(transaction).Error
}

func (r *transactionRepository) IsDuplicate(workspaceID uint, amount float64, merchant string, date time.Time) (bool, error) {
	var count int64

	// 5-minute tolerance window
	startTime := date.Add(-5 * time.Minute)
	endTime := date.Add(5 * time.Minute)

	err := r.db.Model(&models.Transaction{}).
		Where("workspace_id = ?", workspaceID).
		Where("amount = ?", amount).
		Where("merchant = ?", merchant).
		Where("date BETWEEN ? AND ?", startTime, endTime).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (r *transactionRepository) UpdateStatus(id uint, status string) error {
	return r.db.Model(&models.Transaction{}).Where("id = ?", id).Update("status", status).Error
}

func (r *transactionRepository) GetByWorkspaceID(workspaceID uint) ([]models.Transaction, error) {
	var transactions []models.Transaction
	// Order by most recent
	err := r.db.Where("workspace_id = ?", workspaceID).Order("date desc").Find(&transactions).Error
	return transactions, err
}

func (r *transactionRepository) Delete(id uint) error {
	return r.db.Delete(&models.Transaction{}, id).Error
}

func (r *transactionRepository) GetByGmailID(gmailID string) (*models.Transaction, error) {
	var tx models.Transaction
	err := r.db.Where("gmail_id = ?", gmailID).First(&tx).Error
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *transactionRepository) HardDelete(id uint) error {
	return r.db.Unscoped().Delete(&models.Transaction{}, id).Error
}

func (r *transactionRepository) CreateEmailLog(emailLog *models.EmailParsed) error {
	// Prevent duplicate entries based on gmail_id
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "gmail_id"}},
		DoNothing: true,
	}).Create(emailLog).Error
}

func (r *transactionRepository) GetEmailLogByGmailID(gmailID string) (*models.EmailParsed, error) {
	var log models.EmailParsed
	err := r.db.Where("gmail_id = ?", gmailID).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *transactionRepository) GetEmailLogByID(id uint) (*models.EmailParsed, error) {
	var log models.EmailParsed
	err := r.db.First(&log, id).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *transactionRepository) UpdateEmailLogStatus(id uint, status string) error {
	return r.db.Model(&models.EmailParsed{}).Where("id = ?", id).Update("status", status).Error
}

func (r *transactionRepository) GetPendingEmailLogs(userID uint) ([]models.EmailParsed, error) {
	var logs []models.EmailParsed
	err := r.db.Where("user_id = ? AND status = ?", userID, "Pending").Find(&logs).Error
	return logs, err
}

func (r *transactionRepository) GetTotalAmountByType(workspaceID uint, txType string, period string) (float64, error) {
	var total float64
	err := r.db.Model(&models.Transaction{}).
		Where("workspace_id = ? AND type = ? AND to_char(created_at, 'YYYY-MM') = ?", workspaceID, txType, period).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}

func (r *transactionRepository) GetTotalByMonth(workspaceID uint, period string) (float64, error) {
	var total float64
	err := r.db.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("workspace_id = ? AND to_char(date, 'YYYY-MM') = ? AND type = ?", workspaceID, period, "expense").
		Scan(&total).Error

	return total, err
}

func (r *transactionRepository) GetTotalSavings(workspaceID uint, period string) (float64, error) {
	var total float64
	err := r.db.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("workspace_id = ? AND to_char(date, 'YYYY-MM') = ? AND type = ?", workspaceID, period, "savings").
		Scan(&total).Error

	return total, err
}

func (r *transactionRepository) GetEmailParsedByID(id uint) (*models.EmailParsed, error) {
	var emailData models.EmailParsed
	err := r.db.First(&emailData, id).Error
	if err != nil {
		return nil, err
	}
	return &emailData, nil
}

func (r *transactionRepository) DeleteEmailParsed(id uint) error {
	return r.db.Delete(&models.EmailParsed{}, id).Error
}

func (r *transactionRepository) GetSummaryByWorkspace(workspaceID uint, txType string, month string) ([]dto.UserTransactionSummary, error) {
	var summaries []dto.UserTransactionSummary

	err := r.db.Table("transactions").
		Select("transactions.user_id, users.name as user_name, SUM(transactions.amount) as total").
		Joins("JOIN users ON users.id = transactions.user_id").
		Where("transactions.workspace_id = ? AND transactions.type = ? AND TO_CHAR(transactions.date, 'YYYY-MM') = ?",
			workspaceID, txType, month).
		Group("transactions.user_id, users.name").
		Scan(&summaries).Error

	if err != nil {
		return nil, err
	}

	return summaries, nil
}

func (r *transactionRepository) GetTotalByWorkspace(workspaceID uint, txType string, month string) (float64, error) {
	var total float64
	err := r.db.Table("transactions").
		Where("workspace_id = ? AND type = ? AND TO_CHAR(date, 'YYYY-MM') = ?", workspaceID, txType, month).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error
	return total, err
}
