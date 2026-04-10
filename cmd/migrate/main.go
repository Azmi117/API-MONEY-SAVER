package main

import (
	"fmt"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/config"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
)

func main() {
	db := config.ConnectDB()

	err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.EmailParsed{},
		&models.RefreshToken{},
		&models.RevokeToken{},
		&models.Target{},
		&models.Transaction{},
		&models.Workspace{},
		&models.WorkspaceMember{},
		&models.WorkspaceInvitation{},
	)

	if err != nil {
		fmt.Printf("Migration Failed : %s", err)
		return
	}

	fmt.Println("Migration Success!")
}
