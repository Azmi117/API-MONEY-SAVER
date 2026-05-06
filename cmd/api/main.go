package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/config"
	delivery "github.com/Azmi117/API-MONEY-SAVER.git/internal/delivery/http"
	tgDelivery "github.com/Azmi117/API-MONEY-SAVER.git/internal/delivery/telegram" // Import delivery telegram
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/service"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/ocr"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5" // Library Telegram
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Failed load .env!!")
	}

	db := config.ConnectDB()

	ocrKey := os.Getenv("OCR_SPACE_API_KEY")
	ocrClient := ocr.NewOCRSpaceClient(ocrKey)
	pendingRepo := repository.NewPendingTransactionRepository(db)
	targetRepo := repository.NewTargetRepository(db)

	// ---------------------------------------------------------
	// 0. PKG LAYER (External Clients)
	// ---------------------------------------------------------
	geminiApiKey := os.Getenv("GEMINI_API_KEY")
	ctx := context.Background()
	geminiClient, err := gemini.NewGeminiClient(ctx, geminiApiKey)
	if err != nil {
		log.Fatal("Gagal inisialisasi Gemini Client:", err)
	}

	// ---------------------------------------------------------
	// 1. AUTH LAYER
	// ---------------------------------------------------------
	authRepo := repository.NewAuthRepository(db)
	googleAuthService := service.NewGoogleAuthService(authRepo)

	authUsecase := usecase.NewAuthUsecase(authRepo)
	authHandler := delivery.NewAuthHandler(authUsecase, googleAuthService)

	// ---------------------------------------------------------
	// 2. WORKSPACE LAYER
	// ---------------------------------------------------------
	wsRepo := repository.NewWorkspaceRepository(db)
	wsUsecase := usecase.NewWorkspaceUsecase(wsRepo, authRepo)
	wsHandler := delivery.NewWorkspaceHandler(wsUsecase)

	// ---------------------------------------------------------
	// 3. TRANSACTION LAYER
	// ---------------------------------------------------------
	txRepo := repository.NewTransactionRepository(db)
	tesseractClient := ocr.NewTesseractClient()
	hybridScanner := ocr.NewHybridScanner(tesseractClient, geminiClient)

	txUsecase := usecase.NewTransactionUsecase(txRepo, authRepo, googleAuthService, geminiClient, hybridScanner, wsRepo, ocrClient, pendingRepo, targetRepo)
	txHandler := delivery.NewTransactionHandler(txUsecase)

	// ---------------------------------------------------------
	// 4. TELEGRAM BOT LAYER (Muka Baru Mobile Replacement)
	// ---------------------------------------------------------
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Printf("⚠️ Gagal inisialisasi Telegram Bot: %v", err)
	} else {
		// Inisialisasi Handler Telegram
		tgHandler := tgDelivery.NewTelegramHandler(bot, txUsecase, authUsecase, authRepo, wsUsecase, wsRepo, pendingRepo)

		// Jalankan Listener Telegram di Goroutine (Background)
		go func() {
			log.Printf("🤖 [Telegram Bot] Aktif sebagai @%s", bot.Self.UserName)
			tgHandler.Listen()
		}()
	}

	// ---------------------------------------------------------
	// 5. THE ROBOT WORKER (Background Job Gmail)
	// ---------------------------------------------------------
	go func() {
		time.Sleep(10 * time.Second)
		ticker := time.NewTicker(1 * time.Minute)
		fmt.Println("🚀 [Robot Sync] Worker Gmail Aktif! Siap ngecek email tiap 1 menit...")

		for range ticker.C {
			fmt.Println("🤖 [Robot Sync] Scan email mutasi sedang berjalan...")
			ctxSync, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			errSync := txUsecase.SyncGmailTransactions(ctxSync)
			if errSync != nil {
				log.Printf("❌ [Robot Sync] Error: %v\n", errSync)
			}
			cancel()
		}
	}()

	// ---------------------------------------------------------
	// 6. SERVER CONFIG & ROUTES
	// ---------------------------------------------------------
	mux := http.NewServeMux()
	delivery.MapRoutes(mux, authHandler, wsHandler, txHandler, authRepo, db)

	port := ":8080"
	log.Printf("🌍 Server running on port %s", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("Failed running server: %v", err)
	}
}
