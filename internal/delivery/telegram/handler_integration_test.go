package telegram

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/dto"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/repository"
	"github.com/Azmi117/API-MONEY-SAVER.git/internal/usecase"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/gemini"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/ocr"
	"github.com/Azmi117/API-MONEY-SAVER.git/pkg/testutils"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
)

func TestFormatBudgetResponse(t *testing.T) {
	h := &TelegramHandler{}

	t.Run("Nil Data", func(t *testing.T) {
		res := h.FormatBudgetResponse(nil)
		assert.Contains(t, res, "Recorded successfully")
	})

	t.Run("Valid Data", func(t *testing.T) {
		data := &dto.BudgetStatusResponse{
			Period:          "2026-06",
			LimitAmount:     5000000,
			TotalExpense:    1000000,
			RemainingBudget: 4000000,
			ExpenseDetails: []dto.MemberSummary{
				{UserName: "Azmi", Total: 1000000},
			},
		}
		res := h.FormatBudgetResponse(data)
		assert.Contains(t, res, "5000000")
		assert.Contains(t, res, "1000000")
		assert.Contains(t, res, "Azmi")
	})
}

// Helper buat bikin message seolah-olah asli dari Telegram
func createCmdMsg(chatID int64, fromID int64, text string, command string, chatType string) *tgbotapi.Message {
	msg := &tgbotapi.Message{
		Chat: &tgbotapi.Chat{ID: chatID, Type: chatType},
		From: &tgbotapi.User{ID: fromID, FirstName: "Mi"},
		Text: text,
	}
	if command != "" {
		msg.Entities = []tgbotapi.MessageEntity{
			{Type: "bot_command", Offset: 0, Length: len(command) + 1},
		}
	}
	return msg
}

func TestIntegration_TelegramHandler(t *testing.T) {
	db := testutils.SetupTestDB()
	defer testutils.CleanTestDB(db)

	os.MkdirAll("uploads", os.ModePerm)
	defer os.RemoveAll("uploads")

	txRepo := repository.NewTransactionRepository(db)
	authRepo := repository.NewAuthRepository(db)
	wsRepo := repository.NewWorkspaceRepository(db)
	pendingRepo := repository.NewPendingTransactionRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	targetRepo := repository.NewTargetRepository(db)
	debtRepo := repository.NewDebtRepository(db)

	geminiClient := &gemini.GeminiClient{}
	ocrSpaceClient := &ocr.OCRSpaceClient{}
	hybridScanner := &ocr.HybridScanner{}

	targetUsecase := usecase.NewTargetUsecase(targetRepo, txRepo)
	txUsecase := usecase.NewTransactionUsecase(txRepo, authRepo, geminiClient, hybridScanner, wsRepo, ocrSpaceClient, pendingRepo, categoryRepo, targetUsecase)
	pendingUsecase := usecase.NewPendingUsecase(pendingRepo, txRepo, categoryRepo, targetUsecase, txUsecase)
	debtUsecase := usecase.NewDebtUsecase(debtRepo, txRepo)
	authUsecase := usecase.NewAuthUsecase(authRepo, nil, nil)
	wsUsecase := usecase.NewWorkspaceUsecase(wsRepo, authRepo, categoryRepo, targetRepo)

	// 🔥 MOCK TELEGRAM SERVER 🔥
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getFile") {
			if strings.Contains(r.URL.Path, "fail_photo") {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"ok":false,"description":"Mocked failure to get file"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"ok":true,"result":{"file_path":"dummy.jpg"}}`))
			return
		}
		if strings.Contains(r.URL.Path, "/file/") {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte("fake image data"))
			return
		}
		if strings.Contains(r.URL.Path, "getUpdates") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"ok": true,
				"result": [
					{
						"update_id": 1,
						"callback_query": {
							"id": "1",
							"from": {"id": 999},
							"data": "init_ws:split",
							"message": {"chat": {"id": -900, "type": "group"}, "message_id": 10}
						}
					},
					{
						"update_id": 2,
						"message": {
							"message_id": 101,
							"chat": {"id": 999, "type": "private"},
							"from": {"id": 999},
							"text": "/start",
							"entities": [{"type": "bot_command", "offset": 0, "length": 6}]
						}
					},
					{
						"update_id": 3,
						"message": {
							"message_id": 102,
							"chat": {"id": -900, "type": "group"},
							"from": {"id": 999},
							"text": "/info",
							"entities": [{"type": "bot_command", "offset": 0, "length": 5}]
						}
					},
					{
						"update_id": 4,
						"message": {
							"message_id": 103,
							"chat": {"id": 999, "type": "private"},
							"from": {"id": 999},
							"text": "Hello Private"
						}
					},
					{
						"update_id": 5,
						"message": {
							"message_id": 104,
							"chat": {"id": -900, "type": "group"},
							"from": {"id": 999},
							"text": "Makan - 50000"
						}
					}
				]
			}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true,"result":{"message_id": 1}}`))
	}))
	defer mockServer.Close()

	bot, _ := tgbotapi.NewBotAPIWithAPIEndpoint("dummy_token", mockServer.URL+"/%s/%s")

	handler := NewTelegramHandler(
		bot, txUsecase, authUsecase, authRepo, wsUsecase,
		debtUsecase, wsRepo, pendingRepo, pendingUsecase, targetUsecase,
	)

	resetDB := func() { testutils.CleanTestDB(db) }

	t.Run("Coverage: Listen Loop", func(t *testing.T) {
		go handler.Listen()
		time.Sleep(150 * time.Millisecond)
		bot.StopReceivingUpdates()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("Coverage: Private Commands", func(t *testing.T) {
		resetDB()
		user := models.User{Name: "Mi", Email: "priv@test.com", TelegramID: new(int)}
		*user.TelegramID = 111
		db.Create(&user)

		msgGroup := createCmdMsg(-100, 111, "/bind 123", "bind", "group")
		handler.handlePrivateCommands(msgGroup)

		cmds := []string{"bind", "list_workspace", "help", "start"}
		for _, c := range cmds {
			msg := createCmdMsg(111, 111, "/"+c+" XXXXX", c, "private")
			handler.handlePrivateCommands(msg)
		}

		handler.handlePrivateContent(createCmdMsg(111, 111, "Hello", "", "private"))
	})

	t.Run("Coverage: Group Commands", func(t *testing.T) {
		resetDB()
		user := models.User{Name: "Mi", Email: "grp@test.com", TelegramID: new(int)}
		*user.TelegramID = 222
		db.Create(&user)

		chatID := int64(-200)
		ws := models.Workspace{Name: "Grup Split", Type: "split", TelegramChatID: &chatID, OwnerID: user.ID}
		db.Create(&ws)

		msgPriv := createCmdMsg(222, 222, "/init", "init", "private")
		handler.handleGroupCommands(msgPriv)

		msgNotInit := createCmdMsg(-999, 222, "/info", "info", "group")
		handler.handleGroupCommands(msgNotInit)

		debt := models.Debt{WorkspaceID: ws.ID, FromUserID: user.ID, ToUserID: user.ID, Amount: 10000, ShortCode: "AB12", Note: "Ngutang"}
		db.Create(&debt)

		cmds := []string{"init", "help", "info", "cek_utang"}
		for _, c := range cmds {
			msg := createCmdMsg(chatID, 222, "/"+c, c, "group")
			handler.handleGroupCommands(msg)
		}

		msgBayar := createCmdMsg(chatID, 222, "/bayar AB12", "bayar", "group")
		handler.handleGroupCommands(msgBayar)

		ws.Type = "budgeting"
		db.Save(&ws)
		msgInfo := createCmdMsg(chatID, 222, "/info", "info", "group")
		handler.handleGroupCommands(msgInfo)
	})

	t.Run("Coverage: Status Command", func(t *testing.T) {
		resetDB()
		user := models.User{Name: "Mi", Email: "stat@test.com", TelegramID: new(int)}
		*user.TelegramID = 333
		db.Create(&user)

		chatID := int64(-300)
		ws := models.Workspace{Name: "WS", Type: "budgeting", TelegramChatID: &chatID, OwnerID: user.ID}
		db.Create(&ws)
		db.Create(&models.WorkspaceMember{WorkspaceID: ws.ID, UserID: user.ID})
		db.Create(&models.Target{WorkspaceID: ws.ID, Period: time.Now().Format("2006-01"), AmountLimit: 5000000})

		msgPriv := createCmdMsg(333, 333, "/status", "status", "private")
		handler.handleStatus(msgPriv)

		msgGrp := createCmdMsg(chatID, 333, "/status", "status", "group")
		handler.handleStatus(msgGrp)
	})

	t.Run("Coverage: Handle Callback Queries", func(t *testing.T) {
		resetDB()
		user := models.User{Name: "Mi", Email: "cb@test.com", TelegramID: new(int)}
		*user.TelegramID = 444
		db.Create(&user)

		chatID := int64(444)
		ws := models.Workspace{Name: "CB WS", Type: "budgeting", TelegramChatID: &chatID, OwnerID: user.ID}
		db.Create(&ws)
		db.Create(&models.WorkspaceMember{WorkspaceID: ws.ID, UserID: user.ID})
		db.Create(&models.Target{WorkspaceID: ws.ID, Period: time.Now().Format("2006-01"), AmountLimit: 5000000})

		rawTx := `{"amount": 50000, "merchant": "Test", "date": "` + time.Now().Format(time.RFC3339) + `", "type": "expense", "workspace_id": ` + fmt.Sprint(ws.ID) + `, "user_id": ` + fmt.Sprint(user.ID) + `}`

		pt1 := models.PendingTransaction{UserID: user.ID, WorkspaceID: ws.ID, Status: "pending", RawData: rawTx}
		db.Create(&pt1)

		pt2 := models.PendingTransaction{UserID: user.ID, WorkspaceID: ws.ID, Status: "pending", RawData: rawTx}
		db.Create(&pt2)

		cbDatas := []string{
			"init_ws:split",
			"select_alt:dummy.jpg",
			"select_hybrid:dummy.jpg",
			"scan_hybrid:dummy.jpg",
			"scan_alt:dummy.jpg",
			"process_hybrid:dummy.jpg",
			"process_alt:dummy.jpg",
			"method_hybrid:dummy.jpg",
			"method_alt:dummy.jpg",
			"set_type:expense:50000",
			"set_type:fail",
			"confirm_alt:" + fmt.Sprint(pt1.ID),
			"cancel_alt:" + fmt.Sprint(pt1.ID),
			"save_" + fmt.Sprint(pt2.ID),
			"delete_" + fmt.Sprint(pt2.ID),
			"confirm_alt:99999",
			"save_99999",
		}

		for _, data := range cbDatas {
			cb := &tgbotapi.CallbackQuery{
				ID: "1", From: &tgbotapi.User{ID: int64(*user.TelegramID)},
				Data:    data,
				Message: &tgbotapi.Message{Chat: &tgbotapi.Chat{ID: chatID, Title: "Chat"}, MessageID: 10},
			}
			handler.handleCallback(cb)
		}
	})

	t.Run("Coverage: Group Content Text & Photo Download", func(t *testing.T) {
		resetDB()
		user := models.User{Name: "Mi", Email: "content@test.com", TelegramID: new(int)}
		*user.TelegramID = 555
		db.Create(&user)

		chatID := int64(-500)
		ws := models.Workspace{Name: "Budget WS", Type: "budgeting", TelegramChatID: &chatID, OwnerID: user.ID}
		db.Create(&ws)
		db.Create(&models.WorkspaceMember{WorkspaceID: ws.ID, UserID: user.ID})
		db.Create(&models.Target{WorkspaceID: ws.ID, Period: time.Now().Format("2006-01"), AmountLimit: 5000000})

		msg := createCmdMsg(chatID, 555, "", "", "group")

		msg.Photo = []tgbotapi.PhotoSize{{FileID: "dummy_photo"}}
		handler.handleGroupContent(msg)

		msg.Photo = nil
		msg.Text = "Nasi Padang 25000"
		handler.handleGroupContent(msg)

		ws.Type = "split"
		db.Save(&ws)
		msg.Text = ""
		msg.Photo = []tgbotapi.PhotoSize{{FileID: "dummy_photo"}}
		handler.handleGroupContent(msg)
	})

	t.Run("Coverage Boost: Aggressive Branches", func(t *testing.T) {
		resetDB()
		user := models.User{Name: "Cov", Email: "cov@test.com", TelegramID: new(int)}
		*user.TelegramID = 999
		db.Create(&user)

		bindCode := "BIND123"
		userBind := models.User{Name: "Bind", Email: "bind@test.com", BindingCode: &bindCode}
		db.Create(&userBind)

		handler.handleBind(createCmdMsg(999, 999, "/bind BIND123", "bind", "private"))
		handler.handleBind(createCmdMsg(999, 999, "/bind", "bind", "private"))

		msgErrorPriv := createCmdMsg(1234, 1234, "/list_workspace", "list_workspace", "private")
		handler.handlePrivateCommands(msgErrorPriv)

		chatIDBudg := int64(-900)
		wsBudg := models.Workspace{Name: "Budg", Type: "budgeting", TelegramChatID: &chatIDBudg, OwnerID: user.ID}
		db.Create(&wsBudg)
		db.Create(&models.WorkspaceMember{WorkspaceID: wsBudg.ID, UserID: user.ID})
		db.Create(&models.Target{WorkspaceID: wsBudg.ID, Period: time.Now().Format("2006-01"), AmountLimit: 5000000})

		handler.handleGroupCommands(createCmdMsg(chatIDBudg, 999, "/cek_utang", "cek_utang", "group"))
		handler.handleGroupCommands(createCmdMsg(chatIDBudg, 999, "/bayar AB", "bayar", "group"))
		handler.handleGroupCommands(createCmdMsg(chatIDBudg, 999, "/init ABC", "init", "group"))

		handler.handleGroupContent(createCmdMsg(chatIDBudg, 999, "Cuma ngobrol", "", "group"))
		handler.handleGroupContent(createCmdMsg(chatIDBudg, 999, "Makan - 50000", "", "group"))

		msgPhoto := createCmdMsg(chatIDBudg, 999, "", "", "group")
		msgPhoto.Photo = []tgbotapi.PhotoSize{{FileID: "dummy_photo"}}
		handler.handleGroupContent(msgPhoto)

		chatIDSplit := int64(-901)
		wsSplit := models.Workspace{Name: "Split", Type: "split", TelegramChatID: &chatIDSplit, OwnerID: user.ID}
		db.Create(&wsSplit)
		db.Create(&models.WorkspaceMember{WorkspaceID: wsSplit.ID, UserID: user.ID})

		msgPhoto.Chat.ID = chatIDSplit
		handler.handleGroupContent(msgPhoto)

		handler.handleGroupCommands(createCmdMsg(chatIDSplit, 999, "/cek_utang", "cek_utang", "group"))
		handler.handleGroupCommands(createCmdMsg(chatIDSplit, 999, "/bayar", "bayar", "group"))
		handler.handleGroupCommands(createCmdMsg(chatIDSplit, 999, "/bayar XXXX", "bayar", "group"))

		wsNoTarget := models.Workspace{Name: "No Target", Type: "budgeting", TelegramChatID: new(int64), OwnerID: user.ID}
		*wsNoTarget.TelegramChatID = -902
		db.Create(&wsNoTarget)

		// 🔥 SKENARIO AMAN TANPA PANIC COV STATUS 🔥

		// Test Group Bind (sukses & gagal sinkron)
		handler.handleBind(createCmdMsg(-900, 999, fmt.Sprintf("/bind %d", wsBudg.ID), "bind", "group"))
		handler.handleBind(createCmdMsg(-900, 999, "/bind 99999", "bind", "group"))
		handler.handleBind(createCmdMsg(-900, 999, "/bind abc", "bind", "group"))

		// Gagal dapet link foto dari telegram
		msgPhotoFail := createCmdMsg(-900, 999, "", "", "group")
		msgPhotoFail.Photo = []tgbotapi.PhotoSize{{FileID: "fail_photo"}}
		handler.handleGroupContent(msgPhotoFail)
	})
}
