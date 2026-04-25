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
	wsUsecase   usecase.WorkspaceUsecase
	wsRepo      repository.WorkspaceRepository
}

func NewTelegramHandler(bot *tgbotapi.BotAPI, txUsecase usecase.TransactionUsecase, authUsecase usecase.AuthUsecase, authRepo repository.AuthRepository, wsUsecase usecase.WorkspaceUsecase, wsRepo repository.WorkspaceRepository) *TelegramHandler {
	return &TelegramHandler{
		bot:         bot,
		txUsecase:   txUsecase,
		authUsecase: authUsecase,
		authRepo:    authRepo,
		wsUsecase:   wsUsecase,
		wsRepo:      wsRepo,
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

	h.bot.Request(tgbotapi.NewCallback(query.ID, ""))

	var finalStatus string
	if strings.HasPrefix(data, "save_") {
		idStr := strings.TrimPrefix(data, "save_")
		id, _ := strconv.Atoi(idStr)

		err := h.txUsecase.ConfirmTransaction(context.Background(), uint(id))
		if err != nil {
			finalStatus = "❌ Gagal mengonfirmasi transaksi."
		} else {
			finalStatus = "✅ **Berhasil Disimpan!** Transaksi sudah masuk ke pembukuan."
		}
	} else if strings.HasPrefix(data, "delete_") {
		idStr := strings.TrimPrefix(data, "delete_")
		id, _ := strconv.Atoi(idStr)

		err := h.txUsecase.HardDeleteTransaction(uint(id))
		if err != nil {
			finalStatus = "❌ Gagal menghapus data dari sistem."
		} else {
			finalStatus = "🗑️ **Transaksi Dibatalkan.** Data sampah tadi udah gua hapus permanen, cuy."
		}
	}

	editMsg := tgbotapi.NewEditMessageText(chatID, messageID, finalStatus)
	editMsg.ParseMode = "Markdown"
	h.bot.Send(editMsg)
}

func (h *TelegramHandler) handlePhoto(message *tgbotapi.Message) {
	user, err := h.authRepo.GetByTelegramID(int64(message.From.ID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Akun belum terhubung! Silahkan binding lewat Web."))
		return
	}

	var workspaceID uint
	if !message.Chat.IsPrivate() {
		// Jika di GRUP, cari workspace yang terhubung ke Chat ID ini
		ws, err := h.wsRepo.GetByTelegramChatID(message.Chat.ID)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Grup ini belum di-init! Owner harus ketik /init [ID] dulu."))
			return
		}
		workspaceID = ws.ID
	} else {
		// Jika PRIVATE, pakai workspace default (milik sendiri)
		if len(user.OwnedWorkspaces) > 0 {
			workspaceID = user.OwnedWorkspaces[0].ID
		} else {
			h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Lu belum punya Workspace! Bikin dulu di Web, Mi."))
			return
		}
	}

	photos := message.Photo
	fileID := photos[len(photos)-1].FileID
	fileURL, _ := h.bot.GetFileDirectURL(fileID)

	tempFile, err := os.CreateTemp("", "receipt-*.jpg")
	if err != nil {
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	resp, err := http.Get(fileURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Gagal download foto."))
		return
	}
	defer resp.Body.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return
	}

	imgData, _ := os.ReadFile(tempFile.Name())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "⏳ Bentar cuy, Gemini lagi baca struk lu..."))

	result, err := h.txUsecase.ProcessScanHybrid2(ctx, user.ID, workspaceID, imgData, "image/jpeg")
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, "❌ Gagal scan: "+err.Error()))
		return
	}

	response := fmt.Sprintf("🎯 **Hasil Scan Hybrid**\n\nMerchant: %s\nTotal: Rp%.2f\nMetode: %s\n\nSimpan sekarang?",
		result.Transaction.Merchant, result.Transaction.Amount, result.Transaction.Method)

	msg := tgbotapi.NewMessage(message.Chat.ID, response)
	msg.ParseMode = "Markdown"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Simpan", fmt.Sprintf("save_%d", result.Transaction.ID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Hapus", fmt.Sprintf("delete_%d", result.Transaction.ID)),
		),
	)
	msg.ReplyMarkup = keyboard
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
