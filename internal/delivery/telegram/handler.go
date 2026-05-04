package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramHandler struct {
	bot         *tgbotapi.BotAPI
	txUsecase   usecase.TransactionUsecase
	authUsecase usecase.AuthUsecase // Ditambahkan untuk handle binding
	authRepo    repository.AuthRepository
	pendingRepo repository.PendingTransactionRepository
	wsUsecase   usecase.WorkspaceUsecase
	wsRepo      repository.WorkspaceRepository
}

func NewTelegramHandler(bot *tgbotapi.BotAPI, txUsecase usecase.TransactionUsecase, authUsecase usecase.AuthUsecase, authRepo repository.AuthRepository, wsUsecase usecase.WorkspaceUsecase, wsRepo repository.WorkspaceRepository, pendingRepo repository.PendingTransactionRepository) *TelegramHandler {
	return &TelegramHandler{
		bot:         bot,
		txUsecase:   txUsecase,
		authUsecase: authUsecase,
		authRepo:    authRepo,
		wsUsecase:   wsUsecase,
		wsRepo:      wsRepo,
		pendingRepo: pendingRepo,
	}
}

func (h *TelegramHandler) Listen() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := h.bot.GetUpdatesChan(u)

	for update := range updates {
		// --- 1. Handle Callback Query (Tombol Simpan/Hapus) ---
		if update.CallbackQuery != nil {
			h.handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		if update.Message.NewChatMembers != nil {
			for _, member := range update.Message.NewChatMembers {
				// Skip kalau yang masuk itu si Bot sendiri (biar gak nyapa diri sendiri)
				if member.ID == h.bot.Self.ID {
					h.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "🚀 Nesav Bot Aktif! Owner, jangan lupa /init [ID_WORKSPACE] ya."))
					continue
				}

				// Greeting buat member baru
				msgText := fmt.Sprintf("Halo @%s! 👋\n\nBiar bisa nyatet transaksi di grup ini, pastiin:\n1. Email lu udah di-invite Owner ke Web.\n2. Lu udah lakuin /bind di Private Chat gue.", member.UserName)
				h.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msgText))
			}
			continue // Langsung skip ke update berikutnya
		}

		// 1. CEK COMMAND DULU (WAJIB NOMOR SATU)
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "bind":
				h.handleBind(update.Message) // Logic binding bakal jalan di sini
			case "init":
				h.handleInitGroup(update.Message)
			case "list_workspace":
				h.handleListWorkspace(update.Message)
			case "info":
				// Pake method yang beneran udah lu bikin tadi Mi
				ws, err := h.wsRepo.GetByTelegramChatID(update.Message.Chat.ID)
				if err != nil || ws == nil {
					h.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Grup ini belum terhubung ke workspace mana pun, Mi."))
				} else {
					// Tampil sebagai Markdown biar cakep
					res := fmt.Sprintf("📂 **Info Grup Nesav**\n\n📌 **Workspace:** %s\n🆔 **ID:** `%d` \n\nSemua transaksi di grup ini otomatis masuk ke sini.", ws.Name, ws.ID)
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, res)
					msg.ParseMode = "Markdown"
					h.bot.Send(msg)
				}
			case "start":
				h.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Halo Mi!"))
			}
			continue
		}

		// --- 3. Handle Content (Photo/Text) ---
		if update.Message.Photo != nil {
			h.handlePhoto(update.Message)
		} else if update.Message.Text != "" {
			h.handleTextTransaction(update.Message)
		}
	}
}

func (h *TelegramHandler) handleBind(message *tgbotapi.Message) {
	args := message.CommandArguments()
	if args == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, "⚠️ **Mana kodenya cuy?**\nFormat: `/bind NSV-XXXXXX`")
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
		return
	}

	code := strings.TrimSpace(args)
	h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⏳ Lagi nyocokin kode binding lu..."))

	err := h.authUsecase.VerifyAndBindTelegram(int64(message.From.ID), code)
	if err != nil {
		msg := tgbotapi.NewMessage(message.Chat.ID, "❌ **Gagal Binding!**\n"+err.Error())
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "✅ **Mantap Mi!** Akun lu udah terhubung. Sekarang lu bisa nyatet transaksi lewat sini.")
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *TelegramHandler) handleCallback(query *tgbotapi.CallbackQuery) {
	data := query.Data
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	// STEP 1: Identifikasi User dari DB
	user, err := h.authRepo.GetByTelegramID(query.From.ID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Akun belum terhubung, Mi!"))
		return
	}

	// STEP 2: Feedback ke Telegram biar loading di tombol ilang
	h.bot.Request(tgbotapi.NewCallback(query.ID, ""))

	// STEP 3: Handle Pilihan Engine (Hybrid vs Alternatif)
	if strings.HasPrefix(data, "select_alt:") || strings.HasPrefix(data, "select_hybrid:") {
		var fileName string
		isHybrid := strings.HasPrefix(data, "select_hybrid:")

		// Ambil nama file dari callback data
		if isHybrid {
			fileName = strings.TrimPrefix(data, "select_hybrid:")
		} else {
			fileName = strings.TrimPrefix(data, "select_alt:")
		}

		localPath := "uploads/" + fileName

		// STEP 4: Cek keberadaan file (Solusi Anti-Restart)
		// Jika server restart dan file masih ada di folder uploads, proses lanjut.
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ File struk ilang atau sesi basi. Upload lagi ya."))
			return
		}

		// STEP 5: Tentukan Workspace (Group chat vs Private chat)
		var workspaceID uint
		ws, err := h.wsRepo.GetByTelegramChatID(chatID)
		if err == nil && ws != nil {
			workspaceID = ws.ID
		} else if len(user.OwnedWorkspaces) > 0 {
			workspaceID = user.OwnedWorkspaces[0].ID
		}

		// STEP 6: Eksekusi Engine Scan
		if isHybrid {
			// JALUR GEMINI
			h.bot.Send(tgbotapi.NewMessage(chatID, "⏳ Gemini lagi baca struk lu..."))

			imgData, _ := os.ReadFile(localPath)
			result, err := h.txUsecase.ProcessScanHybrid2(context.Background(), user.ID, workspaceID, imgData, "image/jpeg")

			if err != nil {
				h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal Gemini: "+err.Error()))
			} else {
				// Preview untuk Gemini
				response := fmt.Sprintf("🎯 **Hasil Hybrid**\n\nMerchant: %s\nTotal: Rp%.2f\n\nSimpan?",
					result.Transaction.Merchant, result.Transaction.Amount)
				msg := tgbotapi.NewMessage(chatID, response)
				msg.ParseMode = "Markdown"
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Simpan", fmt.Sprintf("save_%d", result.Transaction.ID)),
						tgbotapi.NewInlineKeyboardButtonData("❌ Hapus", fmt.Sprintf("delete_%d", result.Transaction.ID)),
					),
				)
				h.bot.Send(msg)
			}
		} else {
			// JALUR OCR SPACE
			h.bot.Send(tgbotapi.NewMessage(chatID, "⏳ OCR Space lagi proses..."))

			// Kirim localPath ke usecase
			result, pendingID, err := h.txUsecase.ProcessScanAlternative(context.Background(), localPath, user.ID, workspaceID)

			if err != nil {
				h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal OCR: "+err.Error()))
			} else {
				// Preview untuk OCR Space
				text := fmt.Sprintf("🔍 **PREVIEW (OCR)**\n\n🏪 Merchant: %s\n💰 Total: Rp %v\n\nSimpan?",
					result.Transaction.Merchant, result.Transaction.Amount)
				msg := tgbotapi.NewMessage(chatID, text)
				msg.ParseMode = "Markdown"
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Confirm", fmt.Sprintf("confirm_alt:%d", pendingID)),
						tgbotapi.NewInlineKeyboardButtonData("❌ Cancel", fmt.Sprintf("cancel_alt:%d", pendingID)),
					),
				)
				h.bot.Send(msg)
			}
		}

		// STEP 7: Pembersihan (Auto-Clean)
		// File dihapus setelah diproses biar folder uploads gak jebol
		os.Remove(localPath)
		return
	}

	// STEP 8: Jalur Konfirmasi OCR Space (Final Save)
	if strings.HasPrefix(data, "confirm_alt:") {
		idStr := strings.TrimPrefix(data, "confirm_alt:")
		pendingID, _ := strconv.Atoi(idStr)

		err := h.txUsecase.ConfirmPendingTransaction(context.Background(), uint(pendingID))
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal konfirmasi: "+err.Error()))
		} else {
			h.bot.Send(tgbotapi.NewEditMessageText(chatID, messageID, "✅ **Berhasil Disimpan!**"))
		}
		return
	}

	// STEP 9: Jalur Cancel OCR Space (Reject DB)
	if strings.HasPrefix(data, "cancel_alt:") {
		idStr := strings.TrimPrefix(data, "cancel_alt:")
		pendingID, _ := strconv.Atoi(idStr)

		// Update status di database jadi rejected
		h.pendingRepo.UpdateStatus(uint(pendingID), "rejected")

		editMsg := tgbotapi.NewEditMessageText(chatID, messageID, "🗑️ **Scan Dibatalkan.**")
		h.bot.Send(editMsg)
		return
	}

	// STEP 10: Jalur Save/Delete Lama (Hybrid)
	if strings.HasPrefix(data, "save_") {
		idStr := strings.TrimPrefix(data, "save_")
		id, _ := strconv.Atoi(idStr)
		h.txUsecase.ConfirmTransaction(context.Background(), uint(id))
		h.bot.Send(tgbotapi.NewEditMessageText(chatID, messageID, "✅ **Tersimpan!**"))
	} else if strings.HasPrefix(data, "delete_") {
		idStr := strings.TrimPrefix(data, "delete_")
		id, _ := strconv.Atoi(idStr)
		h.txUsecase.HardDeleteTransaction(uint(id))
		h.bot.Send(tgbotapi.NewEditMessageText(chatID, messageID, "🗑️ **Dihapus.**"))
	}
}

func (h *TelegramHandler) handlePhoto(message *tgbotapi.Message) {
	// 1. Validasi User
	_, err := h.authRepo.GetByTelegramID(int64(message.From.ID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Akun belum terhubung!"))
		return
	}

	// 2. Ambil FileID dari Telegram (resolusi tertinggi)
	photos := message.Photo
	fileID := photos[len(photos)-1].FileID
	fileURL, _ := h.bot.GetFileDirectURL(fileID)

	// 3. Download foto ke folder uploads SEKARANG
	resp, _ := http.Get(fileURL)
	defer resp.Body.Close()

	// Gunakan Nanoseconds biar nama file unik & gak gampang ditebak
	fileName := fmt.Sprintf("%d.jpg", time.Now().UnixNano())
	localPath := "uploads/" + fileName

	out, _ := os.Create(localPath)
	io.Copy(out, resp.Body)
	out.Close()

	// 4. Kirim tombol dengan CallbackData berisi NAMA FILE
	msg := tgbotapi.NewMessage(message.Chat.ID, "Pilih metode scan, Mi:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Hybrid (Gemini)", "select_hybrid:"+fileName),
			tgbotapi.NewInlineKeyboardButtonData("Alternatif (OCR Space)", "select_alt:"+fileName),
		),
	)
	h.bot.Send(msg)
}

func (h *TelegramHandler) handleTextTransaction(message *tgbotapi.Message) {
	user, err := h.authRepo.GetByTelegramID(int64(message.From.ID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Akun belum terhubung! Silahkan binding lewat Web."))
		return
	}

	name, amount, ok := ParseChatToTransaction(message.Text)
	if !ok {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Format salah. Coba: 'Nasi Padang 25000'"))
		return
	}

	var workspaceID uint
	if !message.Chat.IsPrivate() {
		ws, err := h.wsRepo.GetByTelegramChatID(message.Chat.ID)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Grup ini belum terhubung ke workspace! Ketik /init [ID] dulu."))
			return
		}
		workspaceID = ws.ID
	} else {
		if len(user.OwnedWorkspaces) > 0 {
			workspaceID = user.OwnedWorkspaces[0].ID
		} else {
			h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Lu belum punya Workspace!"))
			return
		}
	}

	req := dto.CreateTransactionRequest{
		WorkspaceID: workspaceID,
		Amount:      amount,
		Note:        name,
		Type:        "expense",
		Date:        time.Now(),
		Method:      "Telegram",
		Source:      "Telegram",
		GmailID:     fmt.Sprintf("TG-%d", time.Now().UnixNano()),
	}

	err = h.txUsecase.CreateManual(context.Background(), user.ID, req)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Gagal menyimpan transaksi."))
		return
	}

	response := fmt.Sprintf("✅ Berhasil mencatat: %s (Rp%.2f)", name, amount)
	h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, response))
}

func (h *TelegramHandler) handleInitGroup(message *tgbotapi.Message) {
	// 1. Cek apakah ini di Grup atau Private Chat
	if message.Chat.IsPrivate() {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Command ini cuma buat di Grup, cuy!"))
		return
	}

	// 2. Ambil ID Workspace dari argumen (misal: /init 5)
	args := message.CommandArguments()
	wsID, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⚠️ Format salah! Contoh: `/init 5` (5 adalah ID Workspace lu di Web)"))
		return
	}

	// 3. Eksekusi Usecase
	err = h.wsUsecase.InitGroupConnection(int64(message.From.ID), uint(wsID), message.Chat.ID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Gagal: "+err.Error()))
		return
	}

	h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "🎉 **Grup Berhasil Terhubung!**\nSekarang semua transaksi di grup ini bakal masuk ke Workspace ID: "+args))
}

func (h *TelegramHandler) handleListWorkspace(message *tgbotapi.Message) {
	// Sebaiknya ini dipake di Private Chat aja biar list-nya gak keliatan orang lain di grup
	if !message.Chat.IsPrivate() {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Perintah ini cuma bisa di Private Chat bot ya, Mi!"))
		return
	}

	list, err := h.wsUsecase.GetUserWorkspaceList(int64(message.From.ID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Gagal: "+err.Error()))
		return
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, list)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}
