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
	authUsecase usecase.AuthUsecase
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
		if update.CallbackQuery != nil {
			h.handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		// Greeting Member Baru
		if update.Message.NewChatMembers != nil {
			for _, member := range update.Message.NewChatMembers {
				if member.ID == h.bot.Self.ID {
					h.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "🚀 Nesav Bot Aktif! Owner, ketik /init buat aktifin workspace grup ini."))
					continue
				}
				msgText := fmt.Sprintf("Halo @%s! 👋\nPastiin lu udah /bind di Private Chat gue biar bisa nyatet transaksi.", member.UserName)
				h.bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, msgText))
			}
			continue
		}

		// Handle Commands (Mirror Protocol)
		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "bind", "help", "list_workspace", "start":
				h.handlePrivateCommands(update.Message)
			case "init", "info":
				h.handleGroupCommands(update.Message)
			case "status":
				h.handleStatus(update.Message)
			}
			continue
		}

		// Handle Content (Mirror Protocol)
		if update.Message.Chat.IsPrivate() {
			h.handlePrivateContent(update.Message)
		} else {
			h.handleGroupContent(update.Message)
		}
	}
}

// --- PRIVATE CHAT HANDLERS ---
func (h *TelegramHandler) handlePrivateCommands(m *tgbotapi.Message) {
	if !m.Chat.IsPrivate() {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "🔒 Perintah ini hanya bisa di Private Chat bot, Mi!"))
		return
	}

	switch m.Command() {
	case "bind":
		h.handleBind(m)
	case "list_workspace":
		list, err := h.wsUsecase.GetUserWorkspaceList(int64(m.From.ID))
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal: "+err.Error()))
			return
		}
		msg := tgbotapi.NewMessage(m.Chat.ID, list)
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
	case "help":
		helpMsg := "🛠 **Nesav Admin Center**\n\n/bind [kode] - Hubungkan akun Web\n/list_workspace - Daftar Workspace lu\n/status - Summary Global"
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, helpMsg))
	case "start":
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Halo Mi! Bind akun lu dulu pake /bind [kode] yang ada di Web ya."))
	}
}

func (h *TelegramHandler) handlePrivateContent(m *tgbotapi.Message) {
	if m.Text != "" {
		_, _, ok := ParseChatToTransaction(m.Text) // Asumsi fungsi ini global/accessible
		if ok {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "🚫 **Gak boleh nyatet di sini, Mi!**\nBikin grup isinya lu sendiri, terus ketik /init di grup itu buat workspace pribadi."))
			return
		}
	}
	if m.Photo != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "🚫 Jangan kirim struk di sini. Kirim di grup workspace lu."))
	}
}

// --- GROUP CHAT HANDLERS ---
func (h *TelegramHandler) handleGroupCommands(m *tgbotapi.Message) {
	if m.Chat.IsPrivate() {
		return
	}

	switch m.Command() {
	case "init":
		h.handleInitGroup(m)
	case "info":
		ws, err := h.wsRepo.GetByTelegramChatID(m.Chat.ID)
		if err != nil || ws == nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Grup ini belum terhubung ke workspace."))
			return
		}
		res := fmt.Sprintf("📂 **Info Grup**\n📌 **Workspace:** %s\n🆔 **ID:** `%d`", ws.Name, ws.ID)
		msg := tgbotapi.NewMessage(m.Chat.ID, res)
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
	}
}

func (h *TelegramHandler) handleGroupContent(m *tgbotapi.Message) {
	ws, err := h.wsRepo.GetByTelegramChatID(m.Chat.ID)
	if err != nil || ws == nil {
		if m.Text != "" {
			_, _, ok := ParseChatToTransaction(m.Text)
			if ok {
				h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "⚠️ Grup ini belum di-init. Ketik /init dulu Mi!"))
			}
		}
		return
	}

	if m.Photo != nil {
		h.handlePhoto(m)
	} else if m.Text != "" {
		h.handleTextTransaction(m)
	}
}

// --- CORE LOGIC HANDLERS ---
func (h *TelegramHandler) handleStatus(m *tgbotapi.Message) {
	if m.Chat.IsPrivate() {
		list, _ := h.wsUsecase.GetUserWorkspaceList(int64(m.From.ID))
		msg := tgbotapi.NewMessage(m.Chat.ID, "📋 **Global Summary:**\n\n"+list)
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
	} else {
		ws, err := h.wsRepo.GetByTelegramChatID(m.Chat.ID)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "Grup belum terhubung."))
			return
		}
		notification, err := h.txUsecase.CheckWorkspaceTarget(ws.ID)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal tarik data."))
			return
		}
		msg := tgbotapi.NewMessage(m.Chat.ID, notification)
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
	}
}

func (h *TelegramHandler) handleInitGroup(m *tgbotapi.Message) {
	args := m.CommandArguments()
	if strings.TrimSpace(args) == "" {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "⏳ Lagi buatin Workspace otomatis..."))
		ws, err := h.wsUsecase.CreateFromTelegram(context.Background(), int64(m.From.ID), m.Chat.Title, m.Chat.ID)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal: "+err.Error()))
			return
		}
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("🎉 **Workspace Aktif!**\nNama: %s\nID: %d", ws.Name, ws.ID)))
		return
	}

	wsID, _ := strconv.Atoi(strings.TrimSpace(args))
	err := h.wsUsecase.InitGroupConnection(int64(m.From.ID), uint(wsID), m.Chat.ID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal Sinkron: "+err.Error()))
		return
	}
	h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Terhubung ke Workspace ID: "+args))
}

func (h *TelegramHandler) handleBind(m *tgbotapi.Message) {
	args := m.CommandArguments()
	if args == "" {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "⚠️ Format: `/bind NSV-XXXXXX`"))
		return
	}
	err := h.authUsecase.VerifyAndBindTelegram(int64(m.From.ID), strings.TrimSpace(args))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal: "+err.Error()))
		return
	}
	h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ Akun terhubung!"))
}

func (h *TelegramHandler) handleTextTransaction(m *tgbotapi.Message) {
	user, err := h.authRepo.GetByTelegramID(int64(m.From.ID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Akun belum terhubung!"))
		return
	}

	// 1. Panggil UseCase Hybrid buat cek tipe dan ambil nominal
	txType, isFixed, amount := h.txUsecase.ProcessTelegramInput(context.Background(), m.Text)

	if amount == 0 {
		// Jika tidak ada angka, abaikan atau kasih info
		return
	}

	ws, _ := h.wsRepo.GetByTelegramChatID(m.Chat.ID)

	// 2. JALUR CEPAT: Kalau ada tanda + atau -
	if isFixed {
		req := dto.CreateTransactionRequest{
			WorkspaceID: ws.ID,
			Amount:      amount,
			Note:        m.Text, // Pakai teks aslinya aja buat note
			Type:        txType,
			Date:        time.Now(),
			Method:      "Telegram",
			Source:      "telegram",
			GmailID:     fmt.Sprintf("TG-%d", time.Now().UnixNano()),
		}

		notification, err := h.txUsecase.CreateManual(context.Background(), user.ID, req)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal: "+err.Error()))
			return
		}
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, notification))
		return
	}

	// 3. JALUR INTERAKTIF: Kalau polosan, tanya pake Button
	msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("💰 Nominal Rp%.0f. Ini Duit Masuk atau Keluar, Mi?", amount))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			// Simpan metadata di callback data: "pilih_tipe:[type]:[amount]"
			tgbotapi.NewInlineKeyboardButtonData("📥 Income (+)", fmt.Sprintf("set_type:income:%.0f", amount)),
			tgbotapi.NewInlineKeyboardButtonData("📤 Expense (-)", fmt.Sprintf("set_type:expense:%.0f", amount)),
		),
	)
	h.bot.Send(msg)
}

func (h *TelegramHandler) handlePhoto(m *tgbotapi.Message) {
	photos := m.Photo
	fileID := photos[len(photos)-1].FileID
	fileURL, _ := h.bot.GetFileDirectURL(fileID)
	resp, _ := http.Get(fileURL)
	defer resp.Body.Close()

	fileName := fmt.Sprintf("%d.jpg", time.Now().UnixNano())
	localPath := "uploads/" + fileName
	out, _ := os.Create(localPath)
	io.Copy(out, resp.Body)
	out.Close()

	msg := tgbotapi.NewMessage(m.Chat.ID, "Pilih metode scan:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Hybrid (Gemini)", "select_hybrid:"+fileName),
			tgbotapi.NewInlineKeyboardButtonData("Alternatif (OCR Space)", "select_alt:"+fileName),
		),
	)
	h.bot.Send(msg)
}

func (h *TelegramHandler) handleCallback(query *tgbotapi.CallbackQuery) {
	data := query.Data
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	user, err := h.authRepo.GetByTelegramID(query.From.ID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Akun belum terhubung!"))
		return
	}

	// Biar loading di tombol Telegram ilang setelah diklik
	h.bot.Request(tgbotapi.NewCallback(query.ID, ""))

	// --- 1. LOGIC SCAN STRUK (HYBRID/OCR) ---
	if strings.HasPrefix(data, "select_alt:") || strings.HasPrefix(data, "select_hybrid:") {
		var fileName string
		isHybrid := strings.HasPrefix(data, "select_hybrid:")
		if isHybrid {
			fileName = strings.TrimPrefix(data, "select_hybrid:")
		} else {
			fileName = strings.TrimPrefix(data, "select_alt:")
		}

		localPath := "uploads/" + fileName
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ File ilang. Upload lagi Mi."))
			return
		}

		ws, _ := h.wsRepo.GetByTelegramChatID(chatID)
		wsID := ws.ID

		if isHybrid {
			h.bot.Send(tgbotapi.NewMessage(chatID, "⏳ Gemini lagi baca struk..."))
			imgData, _ := os.ReadFile(localPath)
			result, err := h.txUsecase.ProcessScanHybrid2(context.Background(), user.ID, wsID, imgData, "image/jpeg")
			if err != nil {
				h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gemini Gagal: "+err.Error()))
			} else {
				resMsg := fmt.Sprintf("🎯 **Hasil Hybrid**\nMerchant: %s\nTotal: Rp%.2f", result.Transaction.Merchant, result.Transaction.Amount)
				msg := tgbotapi.NewMessage(chatID, resMsg)
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
			h.bot.Send(tgbotapi.NewMessage(chatID, "⏳ OCR Space lagi proses..."))
			result, pendingID, err := h.txUsecase.ProcessScanAlternative(context.Background(), localPath, user.ID, wsID)
			if err != nil {
				h.bot.Send(tgbotapi.NewMessage(chatID, "❌ OCR Gagal: "+err.Error()))
			} else {
				resMsg := fmt.Sprintf("🔍 **PREVIEW (OCR)**\nMerchant: %s\nTotal: Rp %v", result.Transaction.Merchant, result.Transaction.Amount)
				msg := tgbotapi.NewMessage(chatID, resMsg)
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
		os.Remove(localPath)
		return
	}

	// --- 2. LOGIC HYBRID TEXT (INCOME/EXPENSE BUTTON) ---
	if strings.HasPrefix(data, "set_type:") {
		// Format: set_type:income:50000
		parts := strings.Split(data, ":")
		if len(parts) < 3 {
			return
		}
		txType := parts[1]
		amount, _ := strconv.ParseFloat(parts[2], 64)

		ws, _ := h.wsRepo.GetByTelegramChatID(chatID)

		req := dto.CreateTransactionRequest{
			WorkspaceID: ws.ID,
			Amount:      amount,
			Note:        "Input via Telegram Button",
			Type:        txType,
			Date:        time.Now(),
			Method:      "Telegram",
			Source:      "telegram",
			GmailID:     fmt.Sprintf("TG-BTN-%d", time.Now().UnixNano()),
		}

		notification, err := h.txUsecase.CreateManual(context.Background(), user.ID, req)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal simpan: "+err.Error()))
		} else {
			// Edit pesan lama biar tombolnya ilang
			edit := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("✅ **Tersimpan sebagai %s!**", txType))
			edit.ParseMode = "Markdown"
			h.bot.Request(edit)

			// Kirim update sisa jajan (The Guardian)
			h.bot.Send(tgbotapi.NewMessage(chatID, notification))
		}
		return
	}

	// --- 3. LOGIC CONFIRMATION LAINNYA ---
	if strings.HasPrefix(data, "confirm_alt:") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "confirm_alt:"))
		notification, err := h.txUsecase.ConfirmPendingTransaction(context.Background(), uint(id))
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal: "+err.Error()))
		} else {
			h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "✅ **Tersimpan!**"))
			h.bot.Send(tgbotapi.NewMessage(chatID, notification))
		}
	} else if strings.HasPrefix(data, "cancel_alt:") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "cancel_alt:"))
		h.pendingRepo.UpdateStatus(uint(id), "rejected")
		h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "🗑️ **Dibatalkan.**"))
	} else if strings.HasPrefix(data, "save_") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "save_"))
		notification, _ := h.txUsecase.ConfirmTransaction(context.Background(), uint(id))
		h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "✅ **Tersimpan!**"))
		h.bot.Send(tgbotapi.NewMessage(chatID, notification))
	} else if strings.HasPrefix(data, "delete_") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "delete_"))
		h.txUsecase.HardDeleteTransaction(uint(id))
		h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "🗑️ **Dihapus.**"))
	}
}
