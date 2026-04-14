package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/config"
	delivery "github.com/Azmi117/API-MONEY-SAVER.git/internal/delivery/http"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Failed load .env!!")
	}

	db := config.ConnectDB()

	// 0. PKG LAYER (External Clients)
	geminiApiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	geminiClient, err := gemini.NewGeminiClient(ctx, geminiApiKey)

	if err != nil {
		log.Fatal("Gagal inisialisasi Gemini Client:", err)
	}

	// 1. AUTH LAYER
	authRepo := repository.NewAuthRepository(db)
	authUsecase := usecase.NewAuthUsecase(authRepo)
	authHandler := delivery.NewAuthHandler(authUsecase)

	// 2. WORKSPACE LAYER (Inisialisasi komponen baru)
	wsRepo := repository.NewWorkspaceRepository(db)
	wsUsecase := usecase.NewWorkspaceUsecase(wsRepo, authRepo) // Inject authRepo buat logic Tiering
	wsHandler := delivery.NewWorkspaceHandler(wsUsecase)

	// 3. TRANSACTION LAYER (Komponen Baru!)
	txRepo := repository.NewTransactionRepository(db)
	txUsecase := usecase.NewTransactionUsecase(txRepo, geminiClient)
	txHandler := delivery.NewTransactionHandler(txUsecase)

	mux := http.NewServeMux()

	// 4. MAP ROUTES
	// Masukin wsHandler dan db sesuai signature MapRoutes yang baru
	delivery.MapRoutes(mux, authHandler, wsHandler, txHandler, authRepo, db)

	port := ":8080"
	log.Printf("Server running on port %s", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Failed running server: %v", err)
	}
}
