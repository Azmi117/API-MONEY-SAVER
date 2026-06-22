package http

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azmi117/API-MONEY-SAVER.git/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MOCK REPOSITORY
type MockCategoryRepo struct {
	mock.Mock
}

func (m *MockCategoryRepo) Create(cat *models.Category) error {
	args := m.Called(cat)
	return args.Error(0)
}

func (m *MockCategoryRepo) GetByWorkspace(wsID uint) ([]models.Category, error) {
	args := m.Called(wsID)
	return args.Get(0).([]models.Category), args.Error(1)
}

func (m *MockCategoryRepo) FindByID(id uint) (*models.Category, error) {
	args := m.Called(id)
	// Balikin nil kalau error, atau pointer category kalau sukses
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Category), args.Error(1)
}

func TestCategoryHandler(t *testing.T) {
	mockRepo := new(MockCategoryRepo)
	handler := NewCategoryHandler(mockRepo)

	t.Run("Create Category - Success", func(t *testing.T) {
		payload := `{"name": "Makanan", "type": "expense", "icon": "🍔"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/1/categories", bytes.NewBufferString(payload))
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()

		mockRepo.On("Create", mock.Anything).Return(nil).Once()

		handler.Create(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("Create Category - Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{invalid`))
		w := httptest.NewRecorder()

		handler.Create(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetByWorkspace - Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/1/categories", nil)
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()

		mockRepo.On("GetByWorkspace", uint(1)).Return([]models.Category{}, nil).Once()

		handler.GetByWorkspace(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetByWorkspace - Repo Error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()

		mockRepo.On("GetByWorkspace", uint(1)).Return([]models.Category{}, errors.New("db error")).Once()

		handler.GetByWorkspace(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
	// Tambahan Test Case untuk menembak baris yang merah
	t.Run("Create Category - Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil) // Pake GET padahal harus POST
		w := httptest.NewRecorder()

		handler.Create(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Create Category - Repo Error (Internal)", func(t *testing.T) {
		payload := `{"name": "Gagal", "type": "expense"}`
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(payload))
		w := httptest.NewRecorder()

		// Mock return error dari database
		mockRepo.On("Create", mock.Anything).Return(errors.New("db error")).Once()

		handler.Create(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("GetByWorkspace - Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", nil) // Pake POST padahal harus GET
		w := httptest.NewRecorder()

		handler.GetByWorkspace(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}
