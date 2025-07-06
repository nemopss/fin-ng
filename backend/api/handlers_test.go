package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nemopss/fin-ng/backend/db"
	"github.com/nemopss/fin-ng/backend/models"
)

func setupTestHandler(t *testing.T) (*gin.Engine, *db.Storage) {
	if err := godotenv.Load("../.env"); err != nil {
		t.Fatalf("Error loading .env file: %v", err)
	}

	connStr := os.Getenv("POSTGRES_TEST_URL")
	storage, err := db.NewStorage(connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Очистка таблицы перед тестом
	_, err = storage.DB.Exec("TRUNCATE TABLE transactions RESTART IDENTITY")
	if err != nil {
		t.Fatalf("Failed to truncate table: %v", err)
	}

	handler := NewHandler(storage)
	r := gin.Default()
	r.GET("/transactions", handler.GetTransactions)
	r.POST("/transactions", handler.CreateTransaction)
	r.DELETE("/transaction/:id", handler.DeleteTransaction)
	r.GET("/transaction/:id", handler.GetTransaction)

	return r, storage
}

func TestGetTransactions(t *testing.T) {
	r, store := setupTestHandler(t)
	defer store.Close()

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{Amount: 100.50, Type: "income"}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	req, _ := http.NewRequest("GET", "/transactions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var transactions []models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&transactions); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}
	if transactions[0].Amount != 100.50 || transactions[0].Type != "income" {
		t.Errorf("Expected transaction {Amount: 100.50, Type: income}, got %+v", transactions[0])
	}
}

func TestCreateTransaction(t *testing.T) {
	r, store := setupTestHandler(t)
	defer store.Close()

	transaction := models.Transaction{Amount: 200.75, Type: "expense"}
	body, _ := json.Marshal(transaction)
	req, _ := http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var createdTransaction models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&createdTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if createdTransaction.Amount != 200.75 || createdTransaction.Type != "expense" {
		t.Errorf("Expected transaction {Amount: 200.75, Type: expense}, got %+v", createdTransaction)
	}

	// Проверяем, что транзакция сохранена в базе
	transactions, err := store.GetTransactions()
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction in DB, got %d", len(transactions))
	}
}

func TestGetTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{Amount: 300.25, Type: "income"}
	if err := storage.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного получения транзакции
	req, _ := http.NewRequest("GET", "/transaction/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var fetchedTransaction models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&fetchedTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if fetchedTransaction.Amount != 300.25 || fetchedTransaction.Type != "income" {
		t.Errorf("Expected transaction {Amount: 300.25, Type: income}, got %+v", fetchedTransaction)
	}

	// Тест получения несуществующей транзакции
	req, _ = http.NewRequest("GET", "/transaction/999", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{Amount: 400.50, Type: "expense"}
	if err := storage.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного удаления транзакции
	req, _ := http.NewRequest("DELETE", "/transaction/1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Проверяем, что транзакция удалена
	transactions, err := storage.GetTransactions()
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions, got %d", len(transactions))
	}

	// Тест удаления несуществующей транзакции
	req, _ = http.NewRequest("DELETE", "/transaction/999", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
