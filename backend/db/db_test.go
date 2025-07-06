package db

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nemopss/fin-ng/backend/models"
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

	// Очистка таблицы перед тестом
	_, err = store.DB.Exec("TRUNCATE TABLE transactions RESTART IDENTITY")
	if err != nil {
		t.Fatalf("Failed to truncate table: %v", err)
	}

	return store
}

func TestCreateAndGetTransactions(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Тест создания транзакции
	transaction := &models.Transaction{Amount: 200.50, Type: "expense"}
	err := store.CreateTransaction(transaction)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	if transaction.ID == 0 {
		t.Error("Expected transaction ID to be set, got 0")
	}

	// Тест получения транзакций
	transactions, err := store.GetTransactions()
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(transactions))
	}
	if transactions[0].Amount != 200.50 || transactions[0].Type != "expense" {
		t.Errorf("Expected transaction {Amount: 200.50, Type: expense}, got %+v", transactions[0])
	}
}

func TestGetTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестовую транзакцию
	transaction := &models.Transaction{Amount: 300.75, Type: "income"}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного получения транзакции
	fetched, err := store.GetTransaction(transaction.ID)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}
	if fetched == nil {
		t.Error("Expected transaction, got nil")
	}
	if fetched.Amount != 300.75 || fetched.Type != "income" {
		t.Errorf("Expected transaction {Amount: 300.75, Type: income}, got %+v", fetched)
	}

	// Тест получения несуществующей транзакции
	fetched, err = store.GetTransaction(999)
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

	// Создаем тестовую транзакцию
	transaction := &models.Transaction{Amount: 400.50, Type: "expense"}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного удаления
	deleted, err := store.DeleteTransaction(transaction.ID)
	if err != nil {
		t.Fatalf("Failed to delete transaction: %v", err)
	}
	if !deleted {
		t.Error("Expected transaction to be deleted, got false")
	}

	// Проверяем, что транзакция удалена
	transactions, err := store.GetTransactions()
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions, got %d", len(transactions))
	}

	// Тест удаления несуществующей транзакции
	deleted, err = store.DeleteTransaction(999)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if deleted {
		t.Error("Expected no deletion for non-existent transaction, got true")
	}
}
