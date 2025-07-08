package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nemopss/fin-ng/backend/models"
)

type Storage struct {
	DB *sql.DB
}

func NewStorage(connStr string) (*Storage, error) {

	db, err := sql.Open("postgres", connStr)

	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		amount FLOAT,
		type TEXT,
		date TIMESTAMP
	)`)

	if err != nil {
		return nil, err
	}

	return &Storage{DB: db}, nil
}

func (s *Storage) Close() {
	s.DB.Close()
}

func (s *Storage) GetTransactions(filterType string, minAmount, maxAmount float64, sort string) ([]models.Transaction, error) {
	query := "SELECT id, amount, type, date FROM transactions"
	args := []interface{}{}
	var conditions []string

	if filterType != "" {
		if filterType != "income" && filterType != "expense" {
			return nil, fmt.Errorf("invalid type filter: must be 'income' or 'expense'")
		}
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)+1))
		args = append(args, filterType)
	}

	if minAmount > 0 {
		conditions = append(conditions, fmt.Sprintf("amount >= $%d", len(args)+1))
		args = append(args, minAmount)
	}

	if maxAmount > 0 {
		conditions = append(conditions, fmt.Sprintf("amount <= $%d", len(args)+1))
		args = append(args, maxAmount)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if sort == "asc" || sort == "desc" {
		query += fmt.Sprintf(" ORDER BY date %s", sort)
	} else if sort != "" {
		return nil, fmt.Errorf("invalid sort parameter: must be 'asc' or 'desc'")
	}

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}

	var transactions = []models.Transaction{}
	for rows.Next() {
		var t models.Transaction
		err := rows.Scan(&t.ID, &t.Amount, &t.Type, &t.Date)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (s *Storage) GetTransaction(id int) (*models.Transaction, error) {
	var t models.Transaction
	row := s.DB.QueryRow("SELECT id, amount, type, date FROM transactions WHERE id = ($1)", id)
	err := row.Scan(&t.ID, &t.Amount, &t.Type, &t.Date)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (s *Storage) CreateTransaction(t *models.Transaction) error {
	if t.Date.IsZero() {
		t.Date = time.Now()
	}
	return s.DB.QueryRow("INSERT INTO transactions (amount, type, date) VALUES ($1, $2, $3) RETURNING id",
		t.Amount, t.Type, t.Date).
		Scan(&t.ID)
}

func (s *Storage) DeleteTransaction(id int) (bool, error) {
	result, err := s.DB.Exec("DELETE FROM transactions WHERE id = ($1) RETURNING id", id)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

func (s *Storage) UpdateTransaction(t *models.Transaction) (bool, error) {
	result, err := s.DB.Exec("UPDATE transactions SET amount = $1, type = $2, date = $3 WHERE id = $4",
		t.Amount, t.Type, t.Date, t.ID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}
