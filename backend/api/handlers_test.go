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

// setupTestHandler инициализирует тестовую среду, создавая новый роутер Gin и подключение к тестовой базе данных.
// Очищает таблицы перед тестами и настраивает маршруты API с middleware аутентификации.
func setupTestHandler(t *testing.T) (*gin.Engine, *db.Storage) {
	gin.SetMode(gin.ReleaseMode)
	// Загружаем переменные окружения из файла .env
	if err := godotenv.Load("../.env"); err != nil {
		t.Fatalf("Error loading .env file: %v", err)
	}

	// Получаем строку подключения к тестовой базе данных
	connStr := os.Getenv("POSTGRES_TEST_URL")
	storage, err := db.NewStorage(connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Очищаем таблицы transactions, categories, users перед тестами
	_, err = storage.DB.Exec("TRUNCATE TABLE transactions, categories, users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	// Проверяем наличие JWT_SECRET в переменных окружения
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		t.Fatal("JWT_SECRET is required")
	}

	// Создаем новый обработчик с подключением к БД и JWT-секретом
	handler := NewHandler(storage, jwtSecret)
	r := gin.Default()
	// Регистрируем маршруты для регистрации и логина
	r.POST("/register", handler.Register)
	r.POST("/login", handler.Login)

	// Настраиваем защищенные маршруты с middleware аутентификации
	protected := r.Group("/", handler.AuthMiddleware())
	protected.GET("/transactions", handler.GetTransactions)
	protected.POST("/transactions", handler.CreateTransaction)
	protected.GET("/transaction/:id", handler.GetTransaction)
	protected.DELETE("/transaction/:id", handler.DeleteTransaction)
	protected.PUT("/transaction/:id", handler.UpdateTransaction)
	protected.POST("/categories", handler.CreateCategory)
	protected.GET("/categories", handler.GetCategories)
	protected.PUT("/categories/:id", handler.UpdateCategory)
	protected.DELETE("/categories/:id", handler.DeleteCategory)

	return r, storage
}

// getToken выполняет запрос на логин для получения JWT-токена, необходимого для аутентифицированных запросов.
func getToken(t *testing.T, r *gin.Engine, username, password string) string {
	credentials := map[string]string{"username": username, "password": password}
	body, _ := json.Marshal(credentials)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем, что запрос логина успешен
	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var response map[string]string
	// Декодируем ответ для получения токена
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	return response["token"]
}

// TestRegister тестирует функционал регистрации пользователей.
func TestRegister(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Тестируем успешную регистрацию
	user := models.User{Username: "testuser", Password: "password123"}
	body, _ := json.Marshal(user)
	req, _ := http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем, что статус ответа - 201 Created
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response map[string]interface{}
	// Декодируем ответ и проверяем, что имя пользователя совпадает
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["username"] != "testuser" {
		t.Errorf("Expected username 'testuser', got %v", response["username"])
	}

	// Проверяем, что пользователь действительно создан в базе
	fetchedUser, err := storage.GetUserByUsername("testuser")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if fetchedUser == nil {
		t.Error("Expected user, got nil")
	}

	// Тестируем регистрацию с некорректным паролем (слишком короткий)
	user = models.User{Username: "testuser2", Password: "short"}
	body, _ = json.Marshal(user)
	req, _ = http.NewRequest("POST", "/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// TestLogin тестирует функционал логина пользователей.
func TestLogin(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем тестового пользователя
	_, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		// Проверяем, что пользователь успешно создан
		t.Fatalf("Failed to create user: %v", err)
	}

	// Тестируем успешный логин
	credentials := map[string]string{"username": "testuser", "password": "password123"}
	body, _ := json.Marshal(credentials)
	req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем, что статус ответа - 200 OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	// Проверяем, что получен токен
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["token"] == "" {
		t.Error("Expected token, got empty")
	}

	// Тестируем логин с некорректным паролем
	credentials = map[string]string{"username": "testuser", "password": "wrong"}
	body, _ = json.Marshal(credentials)
	req, _ = http.NewRequest("POST", "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestCategories тестирует функционал управления категориями (создание, получение, обновление, удаление).
func TestCategories(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем тестового пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен для аутентификации
	token := getToken(t, r, "testuser", "password123")

	// Тестируем создание категории
	category := models.Category{Name: "food"}
	body, _ := json.Marshal(category)
	req, _ := http.NewRequest("POST", "/categories", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем, что категория создана (201 Created)
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var createdCategory models.Category
	// Проверяем, что созданная категория соответствует ожиданиям
	if err := json.NewDecoder(w.Body).Decode(&createdCategory); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if createdCategory.Name != "food" || createdCategory.UserID != user.ID {
		t.Errorf("Expected category {Name: food, UserID: %d}, got %+v", user.ID, createdCategory)
	}

	// Создаем вторую категорию
	category = models.Category{Name: "transport"}
	body, _ = json.Marshal(category)
	req, _ = http.NewRequest("POST", "/categories", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное создание второй категории
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	// Тестируем получение списка категорий
	req, _ = http.NewRequest("GET", "/categories", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем, что список категорий возвращен (200 OK)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var categories []models.Category
	// Проверяем, что возвращены две категории
	if err := json.NewDecoder(w.Body).Decode(&categories); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(categories) != 2 {
		t.Errorf("Expected 2 categories, got %d", len(categories))
	}
	if categories[0].Name != "food" || categories[1].Name != "transport" {
		t.Errorf("Expected categories [food, transport], got %+v", categories)
	}

	// Тестируем обновление категории
	updatedCategory := models.Category{Name: "groceries"}
	body, _ = json.Marshal(updatedCategory)
	req, _ = http.NewRequest("PUT", "/categories/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное обновление (200 OK)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	// Проверяем, что имя категории обновлено
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if response["name"] != "groceries" {
		t.Errorf("Expected name 'groceries', got %v", response["name"])
	}

	// Тестируем обновление несуществующей категории
	req, _ = http.NewRequest("PUT", "/categories/999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 404 Not Found
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Тестируем удаление категории
	req, _ = http.NewRequest("DELETE", "/categories/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное удаление (204 No Content)
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Проверяем, что осталась одна категория
	req, _ = http.NewRequest("GET", "/categories", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&categories); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if len(categories) != 1 {
		t.Errorf("Expected 1 category, got %d", len(categories))
	}

	// Тестируем удаление категории, используемой в транзакции
	newCategory, err := storage.CreateCategory(user.ID, "entertainment")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}
	transaction := models.Transaction{UserID: user.ID, Amount: 100, Type: "expense", CategoryID: newCategory.ID, Date: time.Now()}
	if err := storage.CreateTransaction(&transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	req, _ = http.NewRequest("DELETE", "/categories/3", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request, так как категория используется
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	var errorResponse gin.H
	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "category is used in transactions" {
		t.Errorf("Expected error 'category is used in transactions', got %v", errorResponse["error"])
	}

	// Тестируем создание категории с пустым именем
	invalidCategory := models.Category{Name: ""}
	body, _ = json.Marshal(invalidCategory)
	req, _ = http.NewRequest("POST", "/categories", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "category name is required" {
		t.Errorf("Expected error 'category name is required', got %v", errorResponse["error"])
	}

	// Тестируем доступ к категориям без токена
	req, _ = http.NewRequest("GET", "/categories", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestCreateTransaction тестирует создание транзакций.
func TestCreateTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем тестового пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Создаем категорию
	category, err := storage.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Тестируем создание транзакции
	transaction := models.Transaction{Amount: 200.75, Type: "expense", CategoryID: category.ID, Date: time.Now()}
	body, _ := json.Marshal(transaction)
	req, _ := http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное создание (201 Created)
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var createdTransaction models.Transaction
	// Проверяем, что транзакция создана с правильными данными
	if err := json.NewDecoder(w.Body).Decode(&createdTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if createdTransaction.UserID != user.ID || createdTransaction.Amount != 200.75 || createdTransaction.Type != "expense" || createdTransaction.CategoryID != category.ID {
		t.Errorf("Expected transaction {UserID: %d, Amount: 200.75, Type: expense, CategoryID: %d}, got %+v", user.ID, category.ID, createdTransaction)
	}

	// Проверяем, что транзакция сохранена в базе
	transactions, total, err := storage.GetTransactions(user.ID, "", 0, 0, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction in DB, got %d", len(transactions))
	}

	// Тестируем создание транзакции без категории
	transactionWithoutCategory := models.Transaction{Amount: 300.00, Type: "income", CategoryID: 0, Date: time.Now()}
	body, _ = json.Marshal(transactionWithoutCategory)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var errorResp gin.H
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResp["error"] != "category_id is required and must be positive" {
		t.Errorf("Expected error 'category_id is required and must be positive', got %v", errorResp["error"])
	}

	// Тестируем создание транзакции с отрицательной суммой
	invalidTransaction := models.Transaction{Amount: -100, Type: "expense", CategoryID: category.ID, Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
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

	// Тестируем создание транзакции с некорректным типом
	invalidTransaction = models.Transaction{Amount: 100, Type: "invalid", CategoryID: category.ID, Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "type must be 'income' or 'expense'" {
		t.Errorf("Expected error 'type must be 'income' or 'expense'', got %v", errorResponse["error"])
	}

	// Тестируем создание транзакции с несуществующей категорией
	invalidTransaction = models.Transaction{Amount: 100, Type: "expense", CategoryID: 999, Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 500 Internal Server Error
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "category does not exist or does not belong to user" {
		t.Errorf("Expected error 'category does not exist or does not belong to user', got %v", errorResponse["error"])
	}

	// Тестируем создание транзакции без токена
	body, _ = json.Marshal(transaction)
	req, _ = http.NewRequest("POST", "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestGetTransactions тестирует получение списка транзакций с различными параметрами фильтрации.
func TestGetTransactions(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем тестового пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Создаем категории
	foodCategory, err := storage.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	transportCategory, err := storage.CreateCategory(user.ID, "transport")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}
	now := time.Now()
	// Создаем тестовые транзакции
	transactions := []models.Transaction{
		{UserID: user.ID, Amount: 100.50, Type: "income", CategoryID: foodCategory.ID, Date: now.Add(-3 * time.Hour)},
		{UserID: user.ID, Amount: 200.75, Type: "expense", CategoryID: transportCategory.ID, Date: now.Add(-2 * time.Hour)},
		{UserID: user.ID, Amount: 300.00, Type: "income", CategoryID: foodCategory.ID, Date: now.Add(-1 * time.Hour)},
		{UserID: user.ID, Amount: 400.25, Type: "expense", CategoryID: transportCategory.ID, Date: now},
	}
	for _, tx := range transactions {
		if err := storage.CreateTransaction(&tx); err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}
	}

	// Тестируем получение транзакций с пагинацией (первая страница)
	req, _ := http.NewRequest("GET", "/transactions?page=1&limit=2", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное получение (200 OK)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response struct {
		Transactions []models.Transaction `json:"transactions"`
		Total        int                  `json:"total"`
	}
	// Проверяем, что возвращено 2 транзакции из 4
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

	// Тестируем вторую страницу
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

	// Тестируем фильтрацию по типу "income"
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

	// Тестируем фильтрацию по категории
	req, _ = http.NewRequest("GET", "/transactions?category_id=1&page=1&limit=1", nil)
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
	if response.Transactions[0].CategoryID != foodCategory.ID {
		t.Errorf("Expected category_id %d, got %d", foodCategory.ID, response.Transactions[0].CategoryID)
	}

	// Тестируем некорректный category_id
	req, _ = http.NewRequest("GET", "/transactions?category_id=-1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Тестируем некорректный тип
	req, _ = http.NewRequest("GET", "/transactions?type=invalid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Тестируем некорректную страницу
	req, _ = http.NewRequest("GET", "/transactions?page=0", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Тестируем превышение лимита
	req, _ = http.NewRequest("GET", "/transactions?limit=101", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	// Тестируем запрос без токена
	req, _ = http.NewRequest("GET", "/transactions", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestGetTransaction тестирует получение конкретной транзакции по ID.
func TestGetTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем тестового пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Создаем категорию
	category, err := storage.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Создаем транзакцию
	transaction := models.Transaction{UserID: user.ID, Amount: 100.50, Type: "income", CategoryID: category.ID, Date: time.Now()}
	if err := storage.CreateTransaction(&transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тестируем получение транзакции
	req, _ := http.NewRequest("GET", "/transaction/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное получение (200 OK)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var fetchedTransaction models.Transaction
	// Проверяем, что данные транзакции совпадают
	if err := json.NewDecoder(w.Body).Decode(&fetchedTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if fetchedTransaction.UserID != user.ID || fetchedTransaction.Amount != 100.50 || fetchedTransaction.Type != "income" || fetchedTransaction.CategoryID != category.ID {
		t.Errorf("Expected transaction {UserID: %d, Amount: 100.50, Type: income, CategoryID: %d}, got %+v", user.ID, category.ID, fetchedTransaction)
	}

	// Тестируем запрос несуществующей транзакции
	req, _ = http.NewRequest("GET", "/transaction/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 404 Not Found
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Тестируем запрос без токена
	req, _ = http.NewRequest("GET", "/transaction/1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestDeleteTransaction тестирует удаление транзакции.
func TestDeleteTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем тестового пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Создаем категорию
	category, err := storage.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Создаем транзакцию
	transaction := models.Transaction{UserID: user.ID, Amount: 100.50, Type: "income", CategoryID: category.ID, Date: time.Now()}
	if err := storage.CreateTransaction(&transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тестируем удаление транзакции
	req, _ := http.NewRequest("DELETE", "/transaction/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное удаление (204 No Content)
	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}

	// Проверяем, что транзакция удалена из базы
	_, total, err := storage.GetTransactions(user.ID, "", 0, 0, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 0 {
		t.Errorf("Expected total 0, got %d", total)
	}

	// Тестируем удаление несуществующей транзакции
	req, _ = http.NewRequest("DELETE", "/transaction/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 404 Not Found
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Тестируем удаление без токена
	req, _ = http.NewRequest("DELETE", "/transaction/1", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// TestUpdateTransaction тестирует обновление транзакции.
func TestUpdateTransaction(t *testing.T) {
	r, storage := setupTestHandler(t)
	defer storage.Close()

	// Создаем тестового пользователя
	user, err := storage.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Получаем токен
	token := getToken(t, r, "testuser", "password123")

	// Создаем категории
	foodCategory, err := storage.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	transportCategory, err := storage.CreateCategory(user.ID, "transport")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Создаем транзакцию
	transaction := models.Transaction{UserID: user.ID, Amount: 100.50, Type: "income", CategoryID: foodCategory.ID, Date: time.Now()}
	if err := storage.CreateTransaction(&transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тестируем обновление транзакции
	updatedTransaction := models.Transaction{Amount: 200.75, Type: "expense", CategoryID: transportCategory.ID, Date: time.Now().Add(time.Hour)}
	body, _ := json.Marshal(updatedTransaction)
	req, _ := http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Проверяем успешное обновление (200 OK)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var fetchedTransaction models.Transaction
	// Проверяем, что данные транзакции обновлены
	if err := json.NewDecoder(w.Body).Decode(&fetchedTransaction); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if fetchedTransaction.UserID != user.ID || fetchedTransaction.Amount != 200.75 || fetchedTransaction.Type != "expense" || fetchedTransaction.CategoryID != transportCategory.ID {
		t.Errorf("Expected transaction {UserID: %d, Amount: 200.75, Type: expense, CategoryID: %d}, got %+v", user.ID, transportCategory.ID, fetchedTransaction)
	}

	// Тестируем обновление с некорректной категорией (CategoryID = 0)
	updatedTransaction = models.Transaction{Amount: 300.00, Type: "income", CategoryID: 0, Date: time.Now().Add(2 * time.Hour)}
	body, _ = json.Marshal(updatedTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var errorResp gin.H
	if err := json.NewDecoder(w.Body).Decode(&errorResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResp["error"] != "category_id is required and must be positive" {
		t.Errorf("Expected error 'category_id is required and must be positive', got %v", errorResp["error"])
	}

	// Тестируем обновление с несуществующей категорией
	invalidTransaction := models.Transaction{Amount: 200.75, Type: "expense", CategoryID: 999, Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 500 Internal Server Error
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var errorResponse gin.H
	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "category does not exist or does not belong to user" {
		t.Errorf("Expected error 'category does not exist or does not belong to user', got %v", errorResponse["error"])
	}

	// Тестируем обновление с отрицательной суммой
	invalidTransaction = models.Transaction{Amount: -100, Type: "expense", CategoryID: foodCategory.ID, Date: time.Now()}
	body, _ = json.Marshal(invalidTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if err := json.NewDecoder(w.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if errorResponse["error"] != "amount must be positive" {
		t.Errorf("Expected error 'amount must be positive', got %v", errorResponse["error"])
	}

	// Тестируем обновление несуществующей транзакции
	body, _ = json.Marshal(updatedTransaction)
	req, _ = http.NewRequest("PUT", "/transaction/999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 404 Not Found
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}

	// Тестируем обновление без токена
	req, _ = http.NewRequest("PUT", "/transaction/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Ожидаем ошибку 401 Unauthorized
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
