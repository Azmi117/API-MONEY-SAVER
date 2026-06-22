package http

import (
	"strconv"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"gorm.io/gorm"
)

// Fungsi ini sekarang bisa diakses sama semua file tes di package http
func seedWorkspaceWithTarget(db *gorm.DB, user models.User, wsName string) models.Workspace {
	ws := models.Workspace{Name: wsName, OwnerID: user.ID}
	db.Create(&ws)
	db.Create(&models.WorkspaceMember{WorkspaceID: ws.ID, UserID: user.ID})
	db.Create(&models.Target{WorkspaceID: ws.ID, Period: time.Now().Format("2006-01"), AmountLimit: 5000000})
	return ws
}

func uintToString(i uint) string {
	return strconv.FormatUint(uint64(i), 10)
}
