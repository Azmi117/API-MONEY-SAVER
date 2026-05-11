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
	debtUsecase usecase.DebtUsecase
	authUsecase usecase.AuthUsecase
	authRepo    repository.AuthRepository
	pendingRepo repository.PendingTransactionRepository
	wsUsecase   usecase.WorkspaceUsecase
	wsRepo      repository.WorkspaceRepository
}

func NewTelegramHandler(bot *tgbotapi.BotAPI, txUsecase usecase.TransactionUsecase, authUsecase usecase.AuthUsecase, authRepo repository.AuthRepository, wsUsecase usecase.WorkspaceUsecase, debtUsecase usecase.DebtUsecase, wsRepo repository.WorkspaceRepository, pendingRepo repository.PendingTransactionRepository) *TelegramHandler {
	return &TelegramHandler{
		bot:         bot,
		txUsecase:   txUsecase,
		debtUsecase: debtUsecase,
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
		// --- 1. HANDLE CALLBACK (BUTTON CLICKS) ---
		if update.CallbackQuery != nil {
			h.handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		// --- 2. GREETING MEMBER BARU ---
		if update.Message.NewChatMembers != nil {
			// (Kode greeting lu tetep di sini)
			continue
		}

		// --- 3. HANDLE COMMANDS ---
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

		// --- 4. HANDLE CONTENT (TEXT/IMAGE) ---
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
	case "help", "start": // Kita satuin ke handleHelp
		h.handleHelp(m)
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
		h.processCekUtang(m, ws.ID) // Pindah ke bawah
	case "bayar":
		if ws.Type != "split" {
			h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "🍕 Fitur ini cuma buat grup **Split Bill**!"))
			return
		}
		h.processBayar(m, ws.ID) // Pindah ke bawah
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

func (h *TelegramHandler) handleSplitPhotoUpload(m *tgbotapi.Message) {
	// 1. Ambil file ID foto (ambil yang ukurannya paling gede/terakhir)
	fileID := m.Photo[len(m.Photo)-1].FileID
	fileURL, err := h.bot.GetFileDirectURL(fileID)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal dapet link foto dari Telegram."))
		return
	}

	// 2. Download dan Simpan ke folder uploads
	fileName := fmt.Sprintf("split_%d_%s.jpg", m.Chat.ID, time.Now().Format("20060102150405"))
	localPath := "uploads/" + fileName

	err = downloadFile(fileURL, localPath)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal simpan foto ke server."))
		return
	}

	// 3. Simpan data awal ke tabel pending_transactions
	// Kita pake UseCase buat nyimpen data "Draft" ini
	ws, _ := h.wsRepo.GetByTelegramChatID(m.Chat.ID)
	user, _ := h.authRepo.GetByTelegramID(int64(m.From.ID))

	// Kita buat record pending dulu biar dapet ID buat jembatan link
	pendingID, err := h.txUsecase.CreatePendingSplit(context.Background(), user.ID, ws.ID, localPath)
	if err != nil {
		h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "❌ Gagal buat draft transaksi: "+err.Error()))
		return
	}

	// 4. Kirim Jembatan Link sakti ke Web Nesav
	// Pakai domain asli atau localhost buat testing
	baseURL := "https://web.nesav.com/split-bill"
	targetURL := fmt.Sprintf("%s/%d", baseURL, pendingID)

	resMsg := fmt.Sprintf("📸 **Struk Terdeteksi (Split Master)!**\n\nStruk lu udah aman di server, Mi. Karena ini grup Split Bill, bagi-bagi itemnya langsung di Web aja ya biar presisi.\n\n🔗 [Klik di sini buat bagi-bagi item](%s)", targetURL)

	msg := tgbotapi.NewMessage(m.Chat.ID, resMsg)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}

// Helper function buat download (taro di paling bawah file handler.go)
func downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
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

	// --- LOGIC: Tipe Workspace Budgeting ---
	if ws.Type == "budgeting" {
		if m.Photo != nil {
			h.handlePhoto(m) // OCR langsung jadi pengeluaran
		} else if m.Text != "" {
			h.handleTextTransaction(m) // "Kopi 15000"
		}
		return
	}

	// --- LOGIC: Tipe Workspace Split ---
	if ws.Type == "split" {
		if m.Photo != nil {
			// Kita bakal buat method baru ini di Step selanjutnya!
			h.handleSplitPhotoUpload(m)
		} else if m.Text != "" {
			// Jika teks biasa di grup split, kita cuekin biar grup gak berisik
			// Kecuali lu mau bot tetep respon kalau formatnya transaksi
			_, _, ok := ParseChatToTransaction(m.Text)
			if ok {
				h.bot.Send(tgbotapi.NewMessage(m.Chat.ID, "🍕 Mi, ini grup Split Bill. Kalau mau nyatet pengeluaran pribadi di grup Budgeting ya!"))
			}
		}
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

	// Skenario 1: /init tanpa argumen (Manual lewat Button)
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

	// Skenario 2: /init [ID] (Link workspace yang udah ada di Web)
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
			edit := tgbotapi.NewEditMessageText(chatID, messageID, fmt.Sprintf("✅ **Tersimpan sebagai %s!**", txType))
			edit.ParseMode = "Markdown"
			h.bot.Request(edit)
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

func (h *TelegramHandler) handleHelp(m *tgbotapi.Message) {
	helpText := `📖 *Panduan Bot Nesav*

*Grup Commands:*
🛡️ /init - Setup grup (Pilih tipe Budgeting atau Split Bill)
📊 /info - Detail Workspace & Status
💸 /cek_utang - Cek tagihan patungan (Khusus Split Bill)
💳 /bayar - Bayar tagihan (Khusus Split Bill)

*Private Commands:*
🔗 /bind [kode] - Hubungkan akun Web
📋 /list_workspace - Daftar Workspace lu
📈 /status - Summary Global keuangan lu

*Tips:* - Di grup *Budgeting*, ketik "Kopi 15000" buat catat jajan.
- Di grup *Split Bill*, upload foto struk buat bagi-bagi tagihan.`

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
