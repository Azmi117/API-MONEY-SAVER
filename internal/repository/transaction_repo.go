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
	// IsDuplicate bakal ngecek kombinasi Amount, Merchant, dan rentang waktu Date
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
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db}
}

func (r *transactionRepository) FindByID(tx *models.Transaction, id uint) error {
	return r.db.First(tx, id).Error
}

func (r *transactionRepository) Create(transaction *models.Transaction) error {
	return r.db.Create(transaction).Error
}

func (r *transactionRepository) IsDuplicate(workspaceID uint, amount float64, merchant string, date time.Time) (bool, error) {
	var count int64

	// Tentukan rentang waktu toleransi (5 menit sebelum & sesudah)
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
	// Kita urutin dari yang paling baru (Descending)
	err := r.db.Where("workspace_id = ?", workspaceID).Order("date desc").Find(&transactions).Error
	return transactions, err
}

func (r *transactionRepository) Delete(id uint) error {
	// Kita pake Unscoped() kalau lu mau bener-bener hapus dari DB (Hard Delete)
	// Kalau di model Transaction lu pake gorm.Model, dia otomatis Soft Delete
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
	// Unscoped() bikin perintah SQL jadi: DELETE FROM transactions WHERE id = ?
	return r.db.Unscoped().Delete(&models.Transaction{}, id).Error
}

func (r *transactionRepository) CreateEmailLog(emailLog *models.EmailParsed) error {
	// TAMBAHIN INI: Biar kalau GmailID duplikat, dia diem aja (DoNothing) gak bikin server error
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
	// Menggunakan to_char untuk mencocokkan CreatedAt dengan format "YYYY-MM"
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
		// UBAH DISINI: Ganti DATE_FORMAT jadi to_char
		Where("workspace_id = ? AND to_char(date, 'YYYY-MM') = ? AND type = ?", workspaceID, period, "expense").
		Scan(&total).Error

	return total, err
}

func (r *transactionRepository) GetTotalSavings(workspaceID uint, period string) (float64, error) {
	var total float64
	err := r.db.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		// UBAH DISINI: Ganti DATE_FORMAT jadi to_char
		Where("workspace_id = ? AND to_char(date, 'YYYY-MM') = ? AND type = ?", workspaceID, period, "savings").
		Scan(&total).Error

	return total, err
}

func (r *transactionRepository) GetEmailParsedByID(id uint) (*models.EmailParsed, error) {
	var emailData models.EmailParsed
	// Nyari data mentah berdasarkan ID yang dikirim dari cegatan Web
	err := r.db.First(&emailData, id).Error
	if err != nil {
		return nil, err
	}
	return &emailData, nil
}

func (r *transactionRepository) DeleteEmailParsed(id uint) error {
	// Hapus data dari table email_parsed supaya gak muncul lagi di list "Pending Approval"
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
