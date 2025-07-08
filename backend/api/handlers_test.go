package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
	r.PUT("/transaction/:id", handler.UpdateTransaction)

	return r, storage
}

func TestGetTransactions(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Добавляем тестовые транзакции
	now := time.Now()
	transactions := []models.Transaction{
		{Amount: 100.50, Type: "income", Date: now.Add(-2 * time.Hour)},
		{Amount: 200.75, Type: "expense", Date: now.Add(-1 * time.Hour)},
		{Amount: 300.00, Type: "income", Date: now},
	}

	for _, tx := range transactions {
		if err := storage.CreateTransaction(&tx); err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}
	}

	req, _ := http.NewRequest("GET", "/transactions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var transactionsResponse []models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&transactionsResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(transactionsResponse) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(transactionsResponse))
	}

	// Тест фильтрации по type
	req, _ = http.NewRequest("GET", "/transactions?type=income", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&transactionsResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(transactionsResponse) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactionsResponse))
	}
	for _, tx := range transactionsResponse {
		if tx.Type != "income" {
			t.Errorf("Expected type 'income', got %s", tx.Type)
		}
	}

	// Тест фильтрации по min_amount
	req, _ = http.NewRequest("GET", "/transactions?min_amount=150", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&transactionsResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(transactionsResponse) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactionsResponse))
	}
	for _, tx := range transactionsResponse {
		if tx.Amount < 150 {
			t.Errorf("Expected amount >= 150, got %f", tx.Amount)
		}
	}

	// Тест фильтрации по max_amount
	req, _ = http.NewRequest("GET", "/transactions?max_amount=250", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&transactionsResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(transactionsResponse) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactionsResponse))
	}
	for _, tx := range transactionsResponse {
		if tx.Amount > 250 {
			t.Errorf("Expected amount <= 250, got %f", tx.Amount)
		}
	}

	// Тест сортировки по date (desc)
	req, _ = http.NewRequest("GET", "/transactions?sort=desc", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&transactionsResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(transactionsResponse) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(transactionsResponse))
	}
	for i := 1; i < len(transactionsResponse); i++ {
		if transactionsResponse[i].Date.After(transactionsResponse[i-1].Date) {
			t.Errorf("Expected transactions sorted by date desc, got %v before %v", transactionsResponse[i].Date, transactionsResponse[i-1].Date)
		}
	}

	// Тест комбинированного фильтра
	req, _ = http.NewRequest("GET", "/transactions?type=income&min_amount=100&max_amount=250&sort=asc", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&transactionsResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(transactionsResponse) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactionsResponse))
	}
	if transactionsResponse[0].Amount != 100.50 || transactionsResponse[0].Type != "income" {
		t.Errorf("Expected transaction {Amount: 100.50, Type: income}, got %+v", transactionsResponse[0])
	}

	// Тест неверного type
	req, _ = http.NewRequest("GET", "/transactions?type=invalid", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Тест неверного min_amount
	req, _ = http.NewRequest("GET", "/transactions?min_amount=invalid", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Тест неверного sort
	req, _ = http.NewRequest("GET", "/transactions?sort=invalid", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	transaction := models.Transaction{Amount: 200.75, Type: "expense", Date: time.Now()}
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
	transactions, err := storage.GetTransactions("", 0, 0, "")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction in DB, got %d", len(transactions))
	}

	// Тест валидации: неверный amount
	invalidTransaction := models.Transaction{Amount: -100, Type: "expense"}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	var errorResponse gin.H
	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "amount must be positive" {
		t.Errorf("Expected error 'amount must be positive', got %v", errorResponse["error"])
	}

	// Тест валидации: неверный type
	invalidTransaction = models.Transaction{Amount: 100, Type: "invalid"}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "type must be 'income' or 'expense'" {
		t.Errorf("Expected error 'type must be 'income' or 'expense'', got %v", errorResponse["error"])
	}

}

func TestGetTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{Amount: 300.25, Type: "income", Date: time.Now()}
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
	transaction := &models.Transaction{Amount: 400.50, Type: "expense", Date: time.Now()}
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
	transactions, err := storage.GetTransactions("", 0, 0, "")
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

func TestUpdateTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{Amount: 500.00, Type: "income", Date: time.Now()}
	if err := storage.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного обновления транзакции
	updatedTransaction := models.Transaction{Amount: 600.25, Type: "expense", Date: time.Now().Add(time.Hour)}
	body, _ := json.Marshal(updatedTransaction)
	req, _ := http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var fetchedTransaction models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&fetchedTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if fetchedTransaction.Amount != 600.25 || fetchedTransaction.Type != "expense" {
		t.Errorf("Expected transaction {Amount: 600.25, Type: expense}, got %+v", fetchedTransaction)
	}
	// Тест валидации: неверный amount
	invalidTransaction := models.Transaction{Amount: -100, Type: "expense", Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var errorResponse gin.H
	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "amount must be positive" {
		t.Errorf("Expected error 'amount must be positive', got %v", errorResponse["error"])
	}

	// Тест валидации: неверный type
	invalidTransaction = models.Transaction{Amount: 100, Type: "invalid", Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "type must be 'income' or 'expense'" {
		t.Errorf("Expected error 'type must be 'income' or 'expense'', got %v", errorResponse["error"])
	}

	// Тест обновления несуществующей транзакции
	body, _ = json.Marshal(updatedTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
