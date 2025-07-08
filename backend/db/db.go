package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/nemopss/fin-ng/backend/models"
	"golang.org/x/crypto/bcrypt"
)

type Storage struct {
	DB *sql.DB
}

func NewStorage(connStr string) (*Storage, error) {

	db, err := sql.Open("postgres", connStr)

	if err != nil {
		return nil, err
	}

	// Создание таблицы users
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username TEXT UNIQUE,
		password TEXT
	)`)
	if err != nil {
		return nil, err
	}

	// Создание таблицы transactions
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS transactions (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
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

func (s *Storage) CreateUser(username, password string) (*models.User, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	if len(password) < 6 {
		return nil, fmt.Errorf("password must be at least 6 characters")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{Username: username, Password: string(hashedPassword)}
	err = s.DB.QueryRow(
		"INSERT INTO users (username, password) VALUES ($1, $2) RETURNING id",
		user.Username, user.Password,
	).Scan(&user.ID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Storage) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	err := s.DB.QueryRow("SELECT id, username, password FROM users WHERE username = $1", username).
		Scan(&user.ID, &user.Username, &user.Password)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Storage) GetTransactions(userID int, filterType string, minAmount, maxAmount float64, sort string) ([]models.Transaction, error) {
	query := "SELECT id, user_id, amount, type, date FROM transactions WHERE user_id = $1"
	args := []interface{}{userID}
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
		query += " AND " + strings.Join(conditions, " AND ")
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
		err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.Type, &t.Date)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (s *Storage) GetTransaction(id, userID int) (*models.Transaction, error) {
	var t models.Transaction
	row := s.DB.QueryRow("SELECT id, user_id, amount, type, date FROM transactions WHERE id = $1 AND user_id = $2", id, userID)
	err := row.Scan(&t.ID, &t.UserID, &t.Amount, &t.Type, &t.Date)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (s *Storage) CreateTransaction(t *models.Transaction) error {
	if t.UserID == 0 {
		return fmt.Errorf("user_id is required")
	}

	if t.Date.IsZero() {
		t.Date = time.Now()
	}
	return s.DB.QueryRow("INSERT INTO transactions (user_id, amount, type, date) VALUES ($1, $2, $3, $4) RETURNING id",
		t.UserID, t.Amount, t.Type, t.Date).
		Scan(&t.ID)
}

func (s *Storage) DeleteTransaction(id, userID int) (bool, error) {
	result, err := s.DB.Exec("DELETE FROM transactions WHERE id = $1 AND user_id = $2 RETURNING id", id, userID)
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
	if t.UserID == 0 {
		return false, fmt.Errorf("user_id is required")
	}
	result, err := s.DB.Exec("UPDATE transactions SET amount = $1, type = $2, date = $3 WHERE id = $4 AND user_id = $5",
		t.Amount, t.Type, t.Date, t.ID, t.UserID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}
