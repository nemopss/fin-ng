package db

import (
	"os"
	"testing"
	"time"

	//	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nemopss/fin-ng/backend/models"
	"golang.org/x/crypto/bcrypt"
)

// setupTestDB инициализирует тестовую базу данных, загружая переменные окружения и создавая новое подключение.
// Очищает таблицы перед тестами для обеспечения чистого состояния.
func setupTestDB(t *testing.T) *Storage {
	// Загружаем переменные окружения из файла .env
	/* if err := godotenv.Load("../.env"); err != nil {
		t.Fatalf("Error loading .env file: %v", err)
	} */

	// Получаем строку подключения к тестовой базе данных
	connStr := os.Getenv("POSTGRES_TEST_URL")
	store, err := NewStorage(connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Очищаем таблицы transactions, categories, users перед тестами
	_, err = store.DB.Exec("TRUNCATE TABLE transactions, categories, users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	return store
}

// TestCreateAndGetUser тестирует создание пользователя и получение его по имени.
func TestCreateAndGetUser(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Тестируем создание пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	// Проверяем, что ID пользователя установлен
	if user.ID == 0 {
		t.Error("Expected user ID to be set, got 0")
	}
	// Проверяем, что имя пользователя совпадает
	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", user.Username)
	}
	// Проверяем, что пароль захеширован корректно
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("password123")); err != nil {
		t.Error("Password hash does not match")
	}

	// Тестируем получение пользователя по имени
	fetchedUser, err := store.GetUserByUsername("testuser")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if fetchedUser == nil {
		t.Error("Expected user, got nil")
	}
	// Проверяем, что данные пользователя совпадают
	if fetchedUser.ID != user.ID || fetchedUser.Username != "testuser" {
		t.Errorf("Expected user {ID: %d, Username: testuser}, got %+v", user.ID, fetchedUser)
	}

	// Тестируем получение несуществующего пользователя
	fetchedUser, err = store.GetUserByUsername("nonexistent")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if fetchedUser != nil {
		t.Errorf("Expected nil user, got %+v", fetchedUser)
	}

	// Тестируем создание пользователя с некорректным паролем (слишком короткий)
	_, err = store.CreateUser("testuser2", "short")
	if err == nil || err.Error() != "password must be at least 6 characters" {
		t.Errorf("Expected error 'password must be at least 6 characters', got %v", err)
	}
}

// TestCategories тестирует функционал управления категориями (создание, получение, обновление, удаление).
func TestCategories(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестового пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Тестируем создание категории
	category, err := store.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}
	// Проверяем, что ID категории установлен
	if category.ID == 0 {
		t.Error("Expected category ID to be set, got 0")
	}
	// Проверяем, что имя категории совпадает
	if category.Name != "food" {
		t.Errorf("Expected name 'food', got %s", category.Name)
	}

	// Тестируем получение категории по ID
	fetched, err := store.GetCategory(category.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to get category: %v", err)
	}
	if fetched == nil {
		t.Error("Expected category, got nil")
	}
	// Проверяем, что данные категории совпадают
	if fetched.ID != category.ID || fetched.Name != "food" {
		t.Errorf("Expected category {ID: %d, Name: food}, got %+v", category.ID, fetched)
	}

	// Тестируем получение списка категорий
	categories, err := store.GetCategories(user.ID)
	if err != nil {
		t.Fatalf("Failed to get categories: %v", err)
	}
	// Проверяем, что возвращена одна категория
	if len(categories) != 1 {
		t.Errorf("Expected 1 category, got %d", len(categories))
	}

	// Тестируем обновление категории
	updated, err := store.UpdateCategory(category.ID, user.ID, "groceries")
	if err != nil {
		t.Fatalf("Failed to update category: %v", err)
	}
	if !updated {
		t.Error("Expected category to be updated, got false")
	}
	// Проверяем, что имя категории обновлено
	fetched, err = store.GetCategory(category.ID, user.ID)
	if fetched.Name != "groceries" {
		t.Errorf("Expected name 'groceries', got %s", fetched.Name)
	}

	// Тестируем удаление категории
	deleted, err := store.DeleteCategory(category.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to delete category: %v", err)
	}
	if !deleted {
		t.Error("Expected category to be deleted, got false")
	}
	// Проверяем, что категория удалена
	fetched, err = store.GetCategory(category.ID, user.ID)
	if fetched != nil {
		t.Errorf("Expected nil category, got %+v", fetched)
	}

	// Тестируем попытку удаления категории, используемой в транзакции
	category, err = store.CreateCategory(user.ID, "transport")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}
	transaction := &models.Transaction{UserID: user.ID, Amount: 100, Type: "expense", CategoryID: category.ID, Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	// Ожидаем ошибку при попытке удаления
	_, err = store.DeleteCategory(category.ID, user.ID)
	if err == nil || err.Error() != "category is used in transactions" {
		t.Errorf("Expected error 'category is used in transactions', got %v", err)
	}
}

// TestCreateAndGetTransactions тестирует создание и получение транзакций.
func TestCreateAndGetTransactions(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестового пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем категорию
	category, err := store.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Тестируем создание транзакции
	transaction := &models.Transaction{UserID: user.ID, Amount: 200.50, Type: "expense", CategoryID: category.ID, Date: time.Now()}
	err = store.CreateTransaction(transaction)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	// Проверяем, что ID транзакции установлен
	if transaction.ID == 0 {
		t.Error("Expected transaction ID to be set, got 0")
	}

	// Тестируем получение транзакций
	transactions, total, err := store.GetTransactions(user.ID, "", 0, 0, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	// Проверяем, что возвращена одна транзакция
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}
	// Проверяем, что данные транзакции совпадают
	if transactions[0].UserID != user.ID || transactions[0].Amount != 200.50 || transactions[0].Type != "expense" || transactions[0].CategoryID != category.ID {
		t.Errorf("Expected transaction {UserID: %d, Amount: 200.50, Type: expense, CategoryID: %d}, got %+v", user.ID, category.ID, transactions[0])
	}
}

// TestGetTransaction тестирует получение конкретной транзакции по ID.
func TestGetTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестового пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем категорию
	category, err := store.CreateCategory(user.ID, "other")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Создаем транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 300.75, Type: "income", CategoryID: category.ID, Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тестируем получение транзакции
	fetched, err := store.GetTransaction(transaction.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}
	if fetched == nil {
		t.Error("Expected transaction, got nil")
	}
	// Проверяем, что данные транзакции совпадают
	if fetched.UserID != user.ID || fetched.Amount != 300.75 || fetched.Type != "income" || fetched.CategoryID != category.ID {
		t.Errorf("Expected transaction {UserID: %d, Amount: 300.75, Type: income, CategoryID: %d}, got %+v", user.ID, category.ID, fetched)
	}

	// Тестируем получение несуществующей транзакции
	fetched, err = store.GetTransaction(999, user.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if fetched != nil {
		t.Errorf("Expected nil transaction, got %+v", fetched)
	}
}

// TestDeleteTransaction тестирует удаление транзакции.
func TestDeleteTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестового пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем категорию
	category, err := store.CreateCategory(user.ID, "transport")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Создаем транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 400.50, Type: "expense", CategoryID: category.ID, Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тестируем удаление транзакции
	deleted, err := store.DeleteTransaction(transaction.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to delete transaction: %v", err)
	}
	if !deleted {
		t.Error("Expected transaction to be deleted, got false")
	}

	// Проверяем, что транзакция удалена
	transactions, total, err := store.GetTransactions(user.ID, "", 0, 0, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 0 {
		t.Errorf("Expected total 0, got %d", total)
	}
	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions, got %d", len(transactions))
	}

	// Тестируем удаление несуществующей транзакции
	deleted, err = store.DeleteTransaction(999, user.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if deleted {
		t.Error("Expected no deletion for non-existent transaction, got true")
	}
}

// TestUpdateTransaction тестирует обновление транзакции.
func TestUpdateTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестового пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем категорию
	category, err := store.CreateCategory(user.ID, "entertainment")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Создаем транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 500.00, Type: "income", CategoryID: category.ID, Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Создаем новую категорию для обновления
	newCategory, err := store.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Тестируем обновление транзакции
	updatedTransaction := &models.Transaction{ID: transaction.ID, UserID: user.ID, Amount: 600.25, Type: "expense", CategoryID: newCategory.ID, Date: time.Now().Add(time.Hour)}
	updated, err := store.UpdateTransaction(updatedTransaction)
	if err != nil {
		t.Fatalf("Failed to update transaction: %v", err)
	}
	if !updated {
		t.Error("Expected transaction to be updated, got false")
	}

	// Проверяем, что транзакция обновлена
	fetched, err := store.GetTransaction(transaction.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}
	if fetched == nil {
		t.Error("Expected transaction, got nil")
	}
	// Проверяем, что данные транзакции совпадают
	if fetched.UserID != user.ID || fetched.Amount != 600.25 || fetched.Type != "expense" || fetched.CategoryID != newCategory.ID {
		t.Errorf("Expected transaction {UserID: %d, Amount: 600.25, Type: expense, CategoryID: %d}, got %+v", user.ID, newCategory.ID, fetched)
	}

	// Тестируем обновление несуществующей транзакции
	nonExistent := &models.Transaction{ID: 999, UserID: user.ID, Amount: 100.00, Type: "income", CategoryID: category.ID, Date: time.Now()}
	updated, err = store.UpdateTransaction(nonExistent)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if updated {
		t.Error("Expected no update for non-existent transaction, got true")
	}
}

// TestGetTransactionsWithFiltersAndPagination тестирует получение транзакций с фильтрами и пагинацией.
func TestGetTransactionsWithFiltersAndPagination(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестового пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем категории
	foodCategory, err := store.CreateCategory(user.ID, "food")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	transportCategory, err := store.CreateCategory(user.ID, "transport")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	// Создаем тестовые транзакции
	now := time.Now()
	transactions := []models.Transaction{
		{UserID: user.ID, Amount: 100.50, Type: "income", CategoryID: foodCategory.ID, Date: now.Add(-3 * time.Hour)},
		{UserID: user.ID, Amount: 200.75, Type: "expense", CategoryID: transportCategory.ID, Date: now.Add(-2 * time.Hour)},
		{UserID: user.ID, Amount: 300.00, Type: "income", CategoryID: foodCategory.ID, Date: now.Add(-1 * time.Hour)},
		{UserID: user.ID, Amount: 400.25, Type: "expense", CategoryID: transportCategory.ID, Date: now},
	}
	for _, tx := range transactions {
		if err := store.CreateTransaction(&tx); err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}
	}

	// Тестируем получение транзакций с пагинацией (первая страница)
	result, total, err := store.GetTransactions(user.ID, "", 0, 0, 0, "asc", 1, 2)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	// Проверяем общее количество транзакций
	if total != 4 {
		t.Errorf("Expected total 4, got %d", total)
	}
	// Проверяем, что возвращены две транзакции
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	// Проверяем суммы транзакций
	if result[0].Amount != 100.50 || result[1].Amount != 200.75 {
		t.Errorf("Expected transactions [100.50, 200.75], got %+v", result)
	}

	// Тестируем вторую страницу
	result, total, err = store.GetTransactions(user.ID, "", 0, 0, 0, "asc", 2, 2)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 4 {
		t.Errorf("Expected total 4, got %d", total)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	if result[0].Amount != 300.00 || result[1].Amount != 400.25 {
		t.Errorf("Expected transactions [300.00, 400.25], got %+v", result)
	}

	// Тестируем фильтрацию по типу "income"
	result, total, err = store.GetTransactions(user.ID, "income", 0, 0, 0, "", 1, 1)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(result))
	}
	if result[0].Type != "income" {
		t.Errorf("Expected type 'income', got %s", result[0].Type)
	}

	// Тестируем фильтрацию по категории
	result, total, err = store.GetTransactions(user.ID, "", foodCategory.ID, 0, 0, "", 1, 1)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(result))
	}
	if result[0].CategoryID != foodCategory.ID {
		t.Errorf("Expected category_id %d, got %d", foodCategory.ID, result[0].CategoryID)
	}

	// Тестируем фильтрацию по минимальной сумме
	result, total, err = store.GetTransactions(user.ID, "", 0, 150, 0, "", 1, 2)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 3 {
		t.Errorf("Expected total 3, got %d", total)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	for _, tx := range result {
		if tx.Amount < 150 {
			t.Errorf("Expected amount >= 150, got %f", tx.Amount)
		}
	}

	// Тестируем сортировку по убыванию
	result, total, err = store.GetTransactions(user.ID, "", 0, 0, 0, "desc", 1, 2)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 4 {
		t.Errorf("Expected total 4, got %d", total)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	if result[0].Amount != 400.25 || result[1].Amount != 300.00 {
		t.Errorf("Expected transactions [400.25, 300.00], got %+v", result)
	}

	// Тестируем комбинированную фильтрацию (тип, категория, сумма)
	result, total, err = store.GetTransactions(user.ID, "income", foodCategory.ID, 100, 250, "asc", 1, 1)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(result))
	}
	if result[0].Amount != 100.50 || result[0].Type != "income" || result[0].CategoryID != foodCategory.ID {
		t.Errorf("Expected transaction {Amount: 100.50, Type: income, CategoryID: %d}, got %+v", foodCategory.ID, result[0])
	}

	// Тестируем некорректный фильтр по типу
	_, _, err = store.GetTransactions(user.ID, "invalid", 0, 0, 0, "", 1, 10)
	if err == nil || err.Error() != "invalid type filter: must be 'income' or 'expense'" {
		t.Errorf("Expected error 'invalid type filter', got %v", err)
	}

	// Тестируем некорректный параметр сортировки
	_, _, err = store.GetTransactions(user.ID, "", 0, 0, 0, "invalid", 1, 10)
	if err == nil || err.Error() != "invalid sort parameter: must be 'asc' or 'desc'" {
		t.Errorf("Expected error 'invalid sort parameter', got %v", err)
	}
}
