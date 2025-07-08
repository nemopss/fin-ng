package db

import (
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nemopss/fin-ng/backend/models"
	"golang.org/x/crypto/bcrypt"
)

func setupTestDB(t *testing.T) *Storage {
	if err := godotenv.Load("../.env"); err != nil {
		t.Fatalf("Error loading .env file: %v", err)
	}

	connStr := os.Getenv("POSTGRES_TEST_URL")
	store, err := NewStorage(connStr)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Очистка таблиц перед тестом
	_, err = store.DB.Exec("TRUNCATE TABLE transactions, users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Failed to truncate tables: %v", err)
	}

	return store
}

func TestCreateAndGetUser(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Тест создания пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	if user.ID == 0 {
		t.Error("Expected user ID to be set, got 0")
	}
	if user.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", user.Username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte("password123")); err != nil {
		t.Error("Password hash does not match")
	}

	// Тест получения пользователя
	fetchedUser, err := store.GetUserByUsername("testuser")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if fetchedUser == nil {
		t.Error("Expected user, got nil")
	}
	if fetchedUser.ID != user.ID || fetchedUser.Username != "testuser" {
		t.Errorf("Expected user {ID: %d, Username: testuser}, got %+v", user.ID, fetchedUser)
	}

	// Тест получения несуществующего пользователя
	fetchedUser, err = store.GetUserByUsername("nonexistent")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if fetchedUser != nil {
		t.Errorf("Expected nil user, got %+v", fetchedUser)
	}

	// Тест валидации: короткий пароль
	_, err = store.CreateUser("testuser2", "short")
	if err == nil || err.Error() != "password must be at least 6 characters" {
		t.Errorf("Expected error 'password must be at least 6 characters', got %v", err)
	}
}

func TestCreateAndGetTransactions(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Тест создания транзакции
	transaction := &models.Transaction{UserID: user.ID, Amount: 200.50, Type: "expense", Date: time.Now()}
	err = store.CreateTransaction(transaction)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	if transaction.ID == 0 {
		t.Error("Expected transaction ID to be set, got 0")
	}

	// Тест получения транзакций
	transactions, total, err := store.GetTransactions(user.ID, "", 0, 0, "", 1, 10)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}
	if transactions[0].UserID != user.ID || transactions[0].Amount != 200.50 || transactions[0].Type != "expense" {
		t.Errorf("Expected transaction {UserID: %d, Amount: 200.50, Type: expense}, got %+v", user.ID, transactions[0])
	}
}

func TestGetTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем тестовую транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 300.75, Type: "income", Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного получения транзакции
	fetched, err := store.GetTransaction(transaction.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}
	if fetched == nil {
		t.Error("Expected transaction, got nil")
	}
	if fetched.UserID != user.ID || fetched.Amount != 300.75 || fetched.Type != "income" {
		t.Errorf("Expected transaction {UserID: %d, Amount: 300.75, Type: income}, got %+v", user.ID, fetched)
	}

	// Тест получения несуществующей транзакции
	fetched, err = store.GetTransaction(999, user.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if fetched != nil {
		t.Errorf("Expected nil transaction, got %+v", fetched)
	}
}

func TestDeleteTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем тестовую транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 400.50, Type: "expense", Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного удаления
	deleted, err := store.DeleteTransaction(transaction.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to delete transaction: %v", err)
	}
	if !deleted {
		t.Error("Expected transaction to be deleted, got false")
	}

	// Проверяем, что транзакция удалена
	transactions, total, err := store.GetTransactions(user.ID, "", 0, 0, "", 1, 10)
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
	deleted, err = store.DeleteTransaction(999, user.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if deleted {
		t.Error("Expected no deletion for non-existent transaction, got true")
	}
}

func TestUpdateTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем тестовую транзакцию
	transaction := &models.Transaction{UserID: user.ID, Amount: 500.00, Type: "income", Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного обновления
	updatedTransaction := &models.Transaction{ID: transaction.ID, UserID: user.ID, Amount: 600.25, Type: "expense", Date: time.Now().Add(time.Hour)}
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
	if fetched.UserID != user.ID || fetched.Amount != 600.25 || fetched.Type != "expense" {
		t.Errorf("Expected transaction {UserID: %d, Amount: 600.25, Type: expense}, got %+v", user.ID, fetched)
	}

	// Тест обновления несуществующей транзакции
	nonExistent := &models.Transaction{ID: 999, UserID: user.ID, Amount: 100.00, Type: "income", Date: time.Now()}
	updated, err = store.UpdateTransaction(nonExistent)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if updated {
		t.Error("Expected no update for non-existent transaction, got true")
	}
}

func TestGetTransactionsWithFiltersAndPagination(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем пользователя
	user, err := store.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Создаем тестовые транзакции
	now := time.Now()
	transactions := []models.Transaction{
		{UserID: user.ID, Amount: 100.50, Type: "income", Date: now.Add(-3 * time.Hour)},
		{UserID: user.ID, Amount: 200.75, Type: "expense", Date: now.Add(-2 * time.Hour)},
		{UserID: user.ID, Amount: 300.00, Type: "income", Date: now.Add(-1 * time.Hour)},
		{UserID: user.ID, Amount: 400.25, Type: "expense", Date: now},
	}
	for _, tx := range transactions {
		if err := store.CreateTransaction(&tx); err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}
	}

	// Тест пагинации: первая страница, limit=2
	result, total, err := store.GetTransactions(user.ID, "", 0, 0, "asc", 1, 2)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 4 {
		t.Errorf("Expected total 4, got %d", total)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	if result[0].Amount != 100.50 || result[1].Amount != 200.75 {
		t.Errorf("Expected transactions [100.50, 200.75], got %+v", result)
	}

	// Тест пагинации: вторая страница, limit=2
	result, total, err = store.GetTransactions(user.ID, "", 0, 0, "asc", 2, 2)
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

	// Тест фильтрации по type с пагинацией
	result, total, err = store.GetTransactions(user.ID, "income", 0, 0, "", 1, 1)
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

	// Тест фильтрации по min_amount с пагинацией
	result, total, err = store.GetTransactions(user.ID, "", 150, 0, "", 1, 2)
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

	// Тест сортировки по date (desc) с пагинацией
	result, total, err = store.GetTransactions(user.ID, "", 0, 0, "desc", 1, 2)
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

	// Тест комбинированного фильтра с пагинацией
	result, total, err = store.GetTransactions(user.ID, "income", 100, 250, "asc", 1, 1)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(result))
	}
	if result[0].Amount != 100.50 || result[0].Type != "income" {
		t.Errorf("Expected transaction {Amount: 100.50, Type: income}, got %+v", result[0])
	}

	// Тест неверного type
	_, _, err = store.GetTransactions(user.ID, "invalid", 0, 0, "", 1, 10)
	if err == nil || err.Error() != "invalid type filter: must be 'income' or 'expense'" {
		t.Errorf("Expected error 'invalid type filter', got %v", err)
	}

	// Тест неверного sort
	_, _, err = store.GetTransactions(user.ID, "", 0, 0, "invalid", 1, 10)
	if err == nil || err.Error() != "invalid sort parameter: must be 'asc' or 'desc'" {
		t.Errorf("Expected error 'invalid sort parameter', got %v", err)
	}
}
