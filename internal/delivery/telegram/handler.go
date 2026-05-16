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
	bot            *tgbotapi.BotAPI
	txUsecase      usecase.TransactionUsecase
	debtUsecase    usecase.DebtUsecase
	authUsecase    usecase.AuthUsecase
	authRepo       repository.AuthRepository
	pendingRepo    repository.PendingTransactionRepository
	wsUsecase      usecase.WorkspaceUsecase
	wsRepo         repository.WorkspaceRepository
	pendingUsecase usecase.PendingUsecase
	targetUsecase  usecase.TargetUsecase
}

func NewTelegramHandler(
	bot *tgbotapi.BotAPI,
	txUsecase usecase.TransactionUsecase,
	authUsecase usecase.AuthUsecase,
	authRepo repository.AuthRepository,
	wsUsecase usecase.WorkspaceUsecase,
	debtUsecase usecase.DebtUsecase,
	wsRepo repository.WorkspaceRepository,
	pendingRepo repository.PendingTransactionRepository,
	pendingUsecase usecase.PendingUsecase,
	targetUsecase usecase.TargetUsecase,
) *TelegramHandler {
	return &TelegramHandler{
		bot:            bot,
		txUsecase:      txUsecase,
		debtUsecase:    debtUsecase,
		authUsecase:    authUsecase,
		authRepo:       authRepo,
		wsUsecase:      wsUsecase,
		wsRepo:         wsRepo,
		pendingRepo:    pendingRepo,
		pendingUsecase: pendingUsecase,
		targetUsecase:  targetUsecase,
	}
}

// --- HELPER: FORMAT DATA DTO KE STRING TELEGRAM ---
func (h *TelegramHandler) FormatBudgetResponse(data *dto.BudgetStatusResponse) string {
	if data == nil {
		return "✅ **Recorded successfully!**"
	}

	res := fmt.Sprintf("📊 **Budget Status for Month %s**\n\n", data.Period)
	res += "💸 **Limit (Expense):**\n"
	res += fmt.Sprintf("🚨 Rp%.2f / Rp%.2f\n", data.TotalExpense, data.LimitAmount)
	res += fmt.Sprintf("Remaining: Rp%.2f\n\n", data.RemainingBudget)

	if len(data.ExpenseDetails) > 0 {
		res += "👤 **Details :**\n"
		for _, s := range data.ExpenseDetails {
			res += fmt.Sprintf("- %s: Rp%.2f\n", s.UserName, s.Total)
		}
		res += "\n"
	}

	return res
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

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "bind", "help", "list_workspace", "start":
				h.handlePrivateCommands(update.Message)
			case "init", "info", "cek_utang", "bayar":
				h.handleGroupCommands(update.Message)
			case "status":
				h.handleStatus(update.Message)
			}
			continue
		}

		if update.Message.Chat.IsPrivate() {
			h.handlePrivateContent(update.Message)
		} else {
			h.handleGroupContent(update.Message)
		}
	}
}

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
	case "help", "start":
		h.handleHelp(m)
	}
}

func (h *TelegramHandler) handlePrivateContent(m *tgbotapi.Message) {
	// Logic content private
}

func (h *TelegramHandler) handleGroupCommands(m *tgbotapi.Message) {
	if m.Chat.IsPrivate() {
		return
	}

	ws, err := h.wsRepo.GetByTelegramChatID(m.Chat.ID)
	if (err != nil || ws == nil) && m.Command() != "init" {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Grup ini belum di-init. Ketik `/init` Mi!"))
		return
	}

	switch m.Command() {
	case "init":
		h.handleInitGroup(m)
	case "help":
		h.handleHelp(m)
	case "info":
		h.handleInfo(m)
	case "cek_utang":
		if ws.Type != "split" {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "🍕 Fitur ini cuma buat grup **Split Bill**!"))
			return
		}
		h.processCekUtang(m, ws.ID)
	case "bayar":
		if ws.Type != "split" {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "🍕 Fitur ini cuma buat grup **Split Bill**!"))
			return
		}
		h.processBayar(m, ws.ID)
	}
}

func (h *TelegramHandler) processCekUtang(m *tgbotapi.Message, wsID uint) {
	debts, err := h.debtUsecase.GetWorkspaceDebts(context.Background(), wsID)
	if err != nil || len(debts) == 0 {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "✅ **Gak ada utang di workspace ini, Mi! Bersih.**"))
		return
	}

	res := "📌 **DAFTAR TAGIHAN BELUM LUNAS**\n"
	res += "--------------------------------------\n"
	for _, d := range debts {
		res += fmt.Sprintf("👤 *@%s*\n💰 Rp%.0f (%s)\n🔑 `/bayar %s` \n\n",
			d.FromUser.Name, d.Amount, d.Note, d.ShortCode)
	}
	res += "--------------------------------------\n_Klik kodenya buat copy otomatis!_"

	msg := tgbotapi.NewMessage(m.Chat.ID, res)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *TelegramHandler) processBayar(m *tgbotapi.Message, wsID uint) {
	shortCode := strings.ToUpper(strings.TrimSpace(m.CommandArguments()))
	if shortCode == "" {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "⚠️ Contoh: `/bayar AB12`"))
		return
	}

	err := h.debtUsecase.ConfirmPayment(context.Background(), wsID, shortCode, int64(m.From.ID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal: "+err.Error()))
		return
	}

	msg := fmt.Sprintf("✅ **LUNAS!**\nKode `%s` udah ditandai lunas ya. Makasih!", shortCode)
	res := tgbotapi.NewMessage(m.Chat.ID, msg)
	res.ParseMode = "Markdown"
	h.bot.Send(res)
}

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
		notification, err := h.targetUsecase.CheckWorkspaceTarget(ws.ID)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal tarik data."))
			return
		}
		msg := tgbotapi.NewMessage(m.Chat.ID, h.FormatBudgetResponse(notification))
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
	}
}

func (h *TelegramHandler) handleInitGroup(m *tgbotapi.Message) {
	args := m.CommandArguments()
	if strings.TrimSpace(args) == "" {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🛡️ Budgeting", "init_ws:budgeting"),
				tgbotapi.NewInlineKeyboardButtonData("🍕 Split Bill", "init_ws:split"),
			),
		)
		msg := tgbotapi.NewMessage(m.Chat.ID, "Halo Mi! Pilih tipe workspace buat grup ini:")
		msg.ReplyMarkup = keyboard
		h.bot.Send(msg)
		return
	}

	wsID, err := strconv.Atoi(strings.TrimSpace(args))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "⚠️ Format salah. Contoh: `/init 123`"))
		return
	}

	err = h.wsUsecase.InitGroupConnection(int64(m.From.ID), uint(wsID), m.Chat.ID)
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

	txType, isFixed, amount := h.txUsecase.ProcessTelegramInput(context.Background(), m.Text)
	if amount == 0 {
		return
	}

	ws, _ := h.wsRepo.GetByTelegramChatID(m.Chat.ID)

	if isFixed {
		req := dto.CreateTransactionRequest{
			WorkspaceID: ws.ID,
			Amount:      amount,
			Note:        m.Text,
			Type:        txType,
			Date:        time.Now(),
			Method:      "Telegram",
			Source:      "telegram",
			GmailID:     fmt.Sprintf("TG-%d", time.Now().UnixNano()),
		}

		// FIX: Tambahin underscore "_" buat nangkep 3 return value
		_, notification, err := h.txUsecase.CreateManual(context.Background(), user.ID, req)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal: "+err.Error()))
			return
		}
		msg := tgbotapi.NewMessage(m.Chat.ID, h.FormatBudgetResponse(notification))
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)
		return
	}

	msg := tgbotapi.NewMessage(m.Chat.ID, fmt.Sprintf("💰 Nominal Rp%.0f. Ini Duit Masuk atau Keluar, Mi?", amount))
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
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

func (h *TelegramHandler) handleSplitPhotoUpload(m *tgbotapi.Message) {
	fileID := m.Photo[len(m.Photo)-1].FileID
	fileURL, err := h.bot.GetFileDirectURL(fileID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal dapet link foto dari Telegram."))
		return
	}

	fileName := fmt.Sprintf("split_%d_%s.jpg", m.Chat.ID, time.Now().Format("20060102150405"))
	localPath := "uploads/" + fileName

	resp, err := http.Get(fileURL)
	if err == nil {
		out, _ := os.Create(localPath)
		io.Copy(out, resp.Body)
		out.Close()
		resp.Body.Close()
	}

	ws, _ := h.wsRepo.GetByTelegramChatID(m.Chat.ID)
	user, _ := h.authRepo.GetByTelegramID(int64(m.From.ID))

	pendingID, err := h.pendingUsecase.CreatePendingSplit(context.Background(), user.ID, ws.ID, localPath)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal buat draft transaksi: "+err.Error()))
		return
	}

	baseURL := "https://web.nesav.com/split-bill"
	targetURL := fmt.Sprintf("%s/%d", baseURL, pendingID)

	resMsg := fmt.Sprintf("📸 **Struk Terdeteksi (Split Master)!**\n\nStruk lu udah aman di server, Mi. Karena ini grup Split Bill, bagi-bagi itemnya langsung di Web aja ya biar presisi.\n\n🔗 [Klik di sini buat bagi-bagi item](%s)", targetURL)

	msg := tgbotapi.NewMessage(m.Chat.ID, resMsg)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *TelegramHandler) handleGroupContent(m *tgbotapi.Message) {
	ws, err := h.wsRepo.GetByTelegramChatID(m.Chat.ID)
	if err != nil || ws == nil {
		return
	}

	if ws.Type == "budgeting" {
		if m.Photo != nil {
			h.handlePhoto(m)
		} else if m.Text != "" {
			h.handleTextTransaction(m)
		}
		return
	}

	if ws.Type == "split" {
		if m.Photo != nil {
			h.handleSplitPhotoUpload(m)
		}
	}
}

func (h *TelegramHandler) handleCallback(query *tgbotapi.CallbackQuery) {
	data := query.Data
	chatID := query.Message.Chat.ID
	messageID := query.Message.MessageID

	// Biar loading di tombol Telegram ilang setelah diklik
	h.bot.Request(tgbotapi.NewCallback(query.ID, ""))

	// --- 0. LOGIC INISIALISASI WORKSPACE (NEW) ---
	if strings.HasPrefix(data, "init_ws:") {
		wsType := strings.Split(data, ":")[1]

		// Panggil UseCase dengan parameter wsType yang baru
		ws, err := h.wsUsecase.CreateFromTelegram(
			context.Background(),
			int64(query.From.ID),
			query.Message.Chat.Title,
			query.Message.Chat.ID,
			wsType,
		)

		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal: "+err.Error()))
			return
		}

		resText := fmt.Sprintf("🎉 **Workspace Aktif!**\nNama: %s\nTipe: %s", ws.Name, wsType)
		edit := tgbotapi.NewEditMessageText(chatID, messageID, resText)
		edit.ParseMode = "Markdown"
		h.bot.Request(edit)
		return
	}

	// Auth Check buat fitur lainnya
	user, err := h.authRepo.GetByTelegramID(int64(query.From.ID))
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Akun belum terhubung!"))
		return
	}

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
			// Kasih info kalau lagi loading biar gak dikira mati
			h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "⏳ Gemini lagi baca struk..."))

			imgData, _ := os.ReadFile(localPath)
			// Asumsi ProcessScanHybrid2 ngeluarin (result, pendingID, err) sesuai fix kita sblmnya
			result, pendingID, err := h.txUsecase.ProcessScanHybrid2(context.Background(), user.ID, wsID, imgData, "image/jpeg")

			if err != nil {
				h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "❌ Gemini Gagal: "+err.Error()))
			} else {
				// Format balikan ke versi asli lu yg detail
				resMsg := fmt.Sprintf("🎯 **Hasil Hybrid**\nMerchant: %s\nTotal: Rp%.2f", result.Transaction.Merchant, result.Transaction.Amount)
				msg := tgbotapi.NewMessage(chatID, resMsg)
				msg.ParseMode = "Markdown"
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
					tgbotapi.NewInlineKeyboardRow(
						tgbotapi.NewInlineKeyboardButtonData("✅ Simpan", fmt.Sprintf("save_%d", pendingID)),
						tgbotapi.NewInlineKeyboardButtonData("❌ Hapus", fmt.Sprintf("delete_%d", pendingID)),
					),
				)
				h.bot.Send(msg)
			}
		} else {
			// Kasih info OCR lagi mikir
			h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "⏳ OCR Space lagi proses..."))

			result, pendingID, err := h.txUsecase.ProcessScanAlternative(context.Background(), localPath, user.ID, wsID)
			if err != nil {
				h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "❌ OCR Gagal: "+err.Error()))
			} else {
				// Format balikan ke versi asli lu yg detail
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

		// FIX 1: Tambahin "_" buat nangkep return value pertama
		_, notification, err := h.txUsecase.CreateManual(context.Background(), user.ID, req)
		if err != nil {
			h.bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal simpan: "+err.Error()))
		} else {
			edit := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("✅ **Tersimpan sebagai %s!**", txType))
			edit.ParseMode = "Markdown"
			h.bot.Request(edit)
			// FIX 3: Bungkus pake h.FormatBudgetResponse
			h.bot.Send(tgbotapi.NewMessage(chatID, h.FormatBudgetResponse(notification)))
		}
		return
	}

	// --- 3. LOGIC CONFIRMATION LAINNYA ---
	if strings.HasPrefix(data, "confirm_alt:") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "confirm_alt:"))

		// FIX: Balikin jadi nangkep 2 variabel aja (notification, err)
		notification, err := h.pendingUsecase.ConfirmPendingTransaction(context.Background(), uint(id))

		if err != nil {
			h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "❌ Gagal: "+err.Error()))
		} else {
			h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "✅ **Tersimpan!**"))
			h.bot.Send(tgbotapi.NewMessage(chatID, h.FormatBudgetResponse(notification)))
		}
	} else if strings.HasPrefix(data, "cancel_alt:") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "cancel_alt:"))
		h.pendingRepo.UpdateStatus(uint(id), "rejected")
		h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "🗑️ **Dibatalkan.**"))
	} else if strings.HasPrefix(data, "save_") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "save_"))

		_, notification, _ := h.txUsecase.ConfirmTransaction(context.Background(), uint(id))
		h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "✅ **Tersimpan!**"))
		// FIX 3: Bungkus pake h.FormatBudgetResponse
		h.bot.Send(tgbotapi.NewMessage(chatID, h.FormatBudgetResponse(notification)))
	} else if strings.HasPrefix(data, "delete_") {
		id, _ := strconv.Atoi(strings.TrimPrefix(data, "delete_"))
		h.txUsecase.HardDeleteTransaction(uint(id))
		h.bot.Request(tgbotapi.NewEditMessageText(chatID, messageID, "🗑️ **Dihapus.**"))
	}
}

func (h *TelegramHandler) handleHelp(m *tgbotapi.Message) {
	helpText := `📖 *Panduan Bot Nesav*

*Grup Commands:*
🛡️ /init - Setup grup
📊 /info - Detail Workspace
💸 /cek_utang - Cek tagihan
💳 /bayar - Bayar tagihan

*Private Commands:*
🔗 /bind [kode] - Hubungkan akun
📋 /list_workspace - Daftar Workspace
📈 /status - Summary Global`

	msg := tgbotapi.NewMessage(m.Chat.ID, helpText)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

func (h *TelegramHandler) handleInfo(m *tgbotapi.Message) {
	ws, _ := h.wsRepo.GetByTelegramChatID(m.Chat.ID)

	var detail string
	if ws.Type == "split" {
		debts, _ := h.debtUsecase.GetWorkspaceDebts(context.Background(), ws.ID)
		detail = fmt.Sprintf("\n🍕 **Tipe:** Split Bill Master\n💰 **Tagihan Aktif:** %d item", len(debts))
	} else {
		// Nanti lu bisa panggil usecase budget di sini Mi
		detail = "\n🛡️ **Tipe:** The Guardian (Budgeting)\n📊 **Status:** Monitoring Aktif"
	}

	res := fmt.Sprintf("📂 **Info Workspace**\n📌 **Nama:** %s\n🆔 **ID:** `%d` %s", ws.Name, ws.ID, detail)
	msg := tgbotapi.NewMessage(m.Chat.ID, res)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}
