package db

import (
	"os"
	"testing"
	"time"

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
	transaction := &models.Transaction{Amount: 200.50, Type: "expense", Date: time.Now()}
	err := store.CreateTransaction(transaction)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	if transaction.ID == 0 {
		t.Error("Expected transaction ID to be set, got 0")
	}

	// Тест получения транзакций
	transactions, err := store.GetTransactions("", 0, 0, "")
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
	transaction := &models.Transaction{Amount: 300.75, Type: "income", Date: time.Now()}
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
	transactions, err := store.GetTransactions("", 0, 0, "")
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

func TestUpdateTransaction(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестовую транзакцию
	transaction := &models.Transaction{Amount: 500.00, Type: "income", Date: time.Now()}
	if err := store.CreateTransaction(transaction); err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}

	// Тест успешного обновления
	updatedTransaction := &models.Transaction{ID: transaction.ID, Amount: 600.25, Type: "expense", Date: time.Now().Add(time.Hour)}
	updated, err := store.UpdateTransaction(updatedTransaction)
	if err != nil {
		t.Fatalf("Failed to update transaction: %v", err)
	}
	if !updated {
		t.Error("Expected transaction to be updated, got false")
	}

	// Проверяем, что транзакция обновлена
	fetched, err := store.GetTransaction(transaction.ID)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}
	if fetched == nil {
		t.Error("Expected transaction, got nil")
	}
	if fetched.Amount != 600.25 || fetched.Type != "expense" {
		t.Errorf("Expected transaction {Amount: 600.25, Type: expense}, got %+v", fetched)
	}

	// Тест обновления несуществующей транзакции
	nonExistent := &models.Transaction{ID: 999, Amount: 100.00, Type: "income", Date: time.Now()}
	updated, err = store.UpdateTransaction(nonExistent)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if updated {
		t.Error("Expected no update for non-existent transaction, got true")
	}
}

func TestGetTransactionsWithFilters(t *testing.T) {
	store := setupTestDB(t)
	defer store.Close()

	// Создаем тестовые транзакции
	now := time.Now()
	transactions := []models.Transaction{
		{Amount: 100.50, Type: "income", Date: now.Add(-2 * time.Hour)},
		{Amount: 200.75, Type: "expense", Date: now.Add(-1 * time.Hour)},
		{Amount: 300.00, Type: "income", Date: now},
	}
	for _, tx := range transactions {
		if err := store.CreateTransaction(&tx); err != nil {
			t.Fatalf("Failed to create transaction: %v", err)
		}
	}

	// Тест фильтрации по type
	result, err := store.GetTransactions("income", 0, 0, "")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	for _, tx := range result {
		if tx.Type != "income" {
			t.Errorf("Expected type 'income', got %s", tx.Type)
		}
	}

	// Тест фильтрации по min_amount
	result, err = store.GetTransactions("", 150, 0, "")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	for _, tx := range result {
		if tx.Amount < 150 {
			t.Errorf("Expected amount >= 150, got %f", tx.Amount)
		}
	}

	// Тест фильтрации по max_amount
	result, err = store.GetTransactions("", 0, 250, "")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(result))
	}
	for _, tx := range result {
		if tx.Amount > 250 {
			t.Errorf("Expected amount <= 250, got %f", tx.Amount)
		}
	}

	// Тест сортировки по date (desc)
	result, err = store.GetTransactions("", 0, 0, "desc")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(result))
	}
	for i := 1; i < len(result); i++ {
		if result[i].Date.After(result[i-1].Date) {
			t.Errorf("Expected transactions sorted by date desc, got %v before %v", result[i].Date, result[i-1].Date)
		}
	}

	// Тест комбинированного фильтра
	result, err = store.GetTransactions("income", 100, 250, "asc")
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("Expected 1 transaction, got %d", len(result))
	}
	if result[0].Amount != 100.50 || result[0].Type != "income" {
		t.Errorf("Expected transaction {Amount: 100.50, Type: income}, got %+v", result[0])
	}

	// Тест неверного type
	_, err = store.GetTransactions("invalid", 0, 0, "")
	if err == nil || err.Error() != "invalid type filter: must be 'income' or 'expense'" {
		t.Errorf("Expected error 'invalid type filter', got %v", err)
	}

	// Тест неверного sort
	_, err = store.GetTransactions("", 0, 0, "invalid")
	if err == nil || err.Error() != "invalid sort parameter: must be 'asc' or 'desc'" {
		t.Errorf("Expected error 'invalid sort parameter', got %v", err)
	}
}
