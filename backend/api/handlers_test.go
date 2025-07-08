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

	// Очистка таблиц перед тестом
	_, err = storage.DB.Exec("TRUNCATE TABLE transactions, users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		t.Fatal("JWT_SECRET is required")
	}

	handler := NewHandler(storage, jwtSecret)
	r := gin.Default()
	r.POST("/register", handler.Register)
	r.POST("/login", handler.Login)

	protected := r.Group("/", handler.AuthMiddleware())
	protected.GET("/transactions", handler.GetTransactions)
	protected.POST("/transactions", handler.CreateTransaction)
	protected.GET("/transaction/:id", handler.GetTransaction)
	protected.DELETE("/transaction/:id", handler.DeleteTransaction)
	protected.PUT("/transaction/:id", handler.UpdateTransaction)

	return r, storage
}

func getToken(t *testing.T, r *gin.Engine, username, password string) string {
	credentials := map[string]string{"username": username, "password": password}
	body, _ := json.Marshal(credentials)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	return response["token"]
}

func TestRegister(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Тест успешной регистрации
	user := models.User{Username: "testuser", Password: "password123"}
	body, _ := json.Marshal(user)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["username"] != "testuser" {
		t.Errorf("Expected username 'testuser', got %v", response["username"])
	}

	// Проверяем, что пользователь сохранен
	fetchedUser, err := storage.GetUserByUsername("testuser")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if fetchedUser == nil {
		t.Error("Expected user, got nil")
	}

	// Тест регистрации с коротким паролем
	user = models.User{Username: "testuser2", Password: "short"}
	body, _ = json.Marshal(user)
	req, _ = http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestLogin(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем пользователя
	_, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Тест успешного входа
	credentials := map[string]string{"username": "testuser", "password": "password123"}
	body, _ := json.Marshal(credentials)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["token"] == "" {
		t.Error("Expected token, got empty")
	}

	// Тест неверного пароля
	credentials = map[string]string{"username": "testuser", "password": "wrong"}
	body, _ = json.Marshal(credentials)
	req, _ = http.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestGetTransactions(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Добавляем тестовые транзакции
	now := time.Now()
	transactions := []models.Transaction{
		{UserID: user.ID, Amount: 100.50, Type: "income", Date: now.Add(-3 * time.Hour)},
		{UserID: user.ID, Amount: 200.75, Type: "expense", Date: now.Add(-2 * time.Hour)},
		{UserID: user.ID, Amount: 300.00, Type: "income", Date: now.Add(-1 * time.Hour)},
		{UserID: user.ID, Amount: 400.25, Type: "expense", Date: now},
	}
	for _, tx := range transactions {
		if err := storage.CreateTransaction(&tx); err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}
	}

	// Тест получения транзакций: первая страница, limit=2
	req, _ := http.NewRequest("GET", "/transactions?page=1&limit=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Transactions []models.Transaction `json:"transactions"`
		Total        int                  `json:"total"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response.Total != 4 {
		t.Errorf("Expected total 4, got %d", response.Total)
	}
	if len(response.Transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(response.Transactions))
	}
	if response.Transactions[0].Amount != 100.50 || response.Transactions[1].Amount != 200.75 {
		t.Errorf("Expected transactions [100.50, 200.75], got %+v", response.Transactions)
	}

	// Тест второй страницы
	req, _ = http.NewRequest("GET", "/transactions?page=2&limit=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response.Total != 4 {
		t.Errorf("Expected total 4, got %d", response.Total)
	}
	if len(response.Transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(response.Transactions))
	}
	if response.Transactions[0].Amount != 300.00 || response.Transactions[1].Amount != 400.25 {
		t.Errorf("Expected transactions [300.00, 400.25], got %+v", response.Transactions)
	}

	// Тест фильтрации по type с пагинацией
	req, _ = http.NewRequest("GET", "/transactions?type=income&page=1&limit=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response.Total != 2 {
		t.Errorf("Expected total 2, got %d", response.Total)
	}
	if len(response.Transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(response.Transactions))
	}
	if response.Transactions[0].Type != "income" {
		t.Errorf("Expected type 'income', got %s", response.Transactions[0].Type)
	}

	// Тест без токена
	req, _ = http.NewRequest("GET", "/transactions", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	// Тест неверного page
	req, _ = http.NewRequest("GET", "/transactions?page=0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Тест неверного limit
	req, _ = http.NewRequest("GET", "/transactions?limit=101", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestCreateTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Тест успешного создания транзакции
	transaction := models.Transaction{Amount: 200.75, Type: "expense", Date: time.Now()}
	body, _ := json.Marshal(transaction)
	req, _ := http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var createdTransaction models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&createdTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if createdTransaction.UserID != user.ID || createdTransaction.Amount != 200.75 || createdTransaction.Type != "expense" {
		t.Errorf("Expected transaction {UserID: %d, Amount: 200.75, Type: expense}, got %+v", user.ID, createdTransaction)
	}

	// Проверяем, что транзакция сохранена в базе
	transactions, total, err := storage.GetTransactions(user.ID, "", 0, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction in DB, got %d", len(transactions))
	}

	// Тест валидации: неверный amount
	invalidTransaction := models.Transaction{Amount: -100, Type: "expense", Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
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
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
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

	// Создаем пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 300.25, Type: "income", Date: time.Now()}
	if err := storage.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного получения транзакции
	req, _ := http.NewRequest("GET", "/transaction/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var fetchedTransaction models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&fetchedTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if fetchedTransaction.UserID != user.ID || fetchedTransaction.Amount != 300.25 || fetchedTransaction.Type != "income" {
		t.Errorf("Expected transaction {UserID: %d, Amount: 300.25, Type: income}, got %+v", user.ID, fetchedTransaction)
	}

	// Тест получения несуществующей транзакции
	req, _ = http.NewRequest("GET", "/transaction/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestDeleteTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 400.50, Type: "expense", Date: time.Now()}
	if err := storage.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного удаления транзакции
	req, _ := http.NewRequest("DELETE", "/transaction/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Проверяем, что транзакция удалена
	transactions, total, err := storage.GetTransactions(user.ID, "", 0, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 0 {
		t.Errorf("Expected total 0, got %d", total)
	}
	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions, got %d", len(transactions))
	}

	// Тест удаления несуществующей транзакции
	req, _ = http.NewRequest("DELETE", "/transaction/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestUpdateTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Добавляем тестовую транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 500.00, Type: "income", Date: time.Now()}
	if err := storage.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного обновления транзакции
	updatedTransaction := models.Transaction{Amount: 600.25, Type: "expense", Date: time.Now().Add(time.Hour)}
	body, _ := json.Marshal(updatedTransaction)
	req, _ := http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var fetchedTransaction models.Transaction
	if err := json.NewDecoder(w.Body).Decode(&fetchedTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if fetchedTransaction.UserID != user.ID || fetchedTransaction.Amount != 600.25 || fetchedTransaction.Type != "expense" {
		t.Errorf("Expected transaction {UserID: %d, Amount: 600.25, Type: expense}, got %+v", user.ID, fetchedTransaction)
	}

	// Тест валидации: неверный amount
	invalidTransaction := models.Transaction{Amount: -100, Type: "expense", Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
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
	req.Header.Set("Authorization", "Bearer "+token)
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
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}
