package main

import (
	"log"
	"net/http"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/config"
	delivery "github.com/Azmi117/API-MONEY-SAVER.git/internal/delivery/http"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Failed load .env!")
	}

	db := config.ConnectDB()

	authRepo := repository.NewAuthRepository(db)
	authUsecase := usecase.NewAuthUsecase(authRepo)
	authHandler := delivery.NewAuthHandler(authUsecase)

	mux := http.NewServeMux()

	delivery.MapRoutes(mux, authHandler, authRepo)

	port := ":8080"
	log.Printf("Server running on port %s", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Failed running server: %v", err)
	}
}
