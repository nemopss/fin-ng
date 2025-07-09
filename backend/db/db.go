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

	// Создание таблицы categories
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS categories (
		id SERIAL PRIMARY KEY,
		user_id INTEGER REFERENCES users(id),
		name TEXT NOT NULL
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
		category_id INTEGER REFERENCES categories(id),
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

func (s *Storage) CreateCategory(userID int, name string) (*models.Category, error) {
	if name == "" {
		return nil, fmt.Errorf("category name is required")
	}

	category := &models.Category{UserID: userID, Name: name}
	err := s.DB.QueryRow("INSERT INTO categories (user_id, name) VALUES ($1, $2) RETURNING id", userID, name).Scan(&category.ID)
	if err != nil {
		return nil, err
	}

	return category, nil
}

func (s *Storage) GetCategories(userID int) ([]models.Category, error) {
	rows, err := s.DB.Query("SELECT id, user_id, name FROM categories WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

func (s *Storage) GetCategory(id, userID int) (*models.Category, error) {
	var c models.Category
	err := s.DB.QueryRow("SELECT id, user_id, name FROM categories WHERE id = $1 AND user_id = $2", id, userID).Scan(&c.ID, &c.UserID, &c.Name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &c, nil
}

func (s *Storage) UpdateCategory(id, userID int, name string) (bool, error) {
	if name == "" {
		return false, fmt.Errorf("category name is required")
	}

	result, err := s.DB.Exec("UPDATE categories SET name = $1 WHERE id = $2 AND user_id = $3", name, id, userID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil

}

func (s *Storage) DeleteCategory(id, userID int) (bool, error) {
	var count int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM transactions WHERE category_id = $1 AND user_id = $2", id, userID).Scan(&count)
	if count > 0 {
		return false, fmt.Errorf("category is used in transactions")
	}
	if err != nil {
		return false, err
	}

	result, err := s.DB.Exec("DELETE FROM categories WHERE id = $1 AND user_id = $2", id, userID)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil

}

func (s *Storage) GetTransactions(userID int, filterType string, filterCategoryID int, minAmount, maxAmount float64, sort string, page, limit int) ([]models.Transaction, int, error) {
	countQuery := "SELECT COUNT(*) FROM transactions WHERE user_id = $1"
	args := []interface{}{userID}
	var conditions []string

	if filterType != "" {
		if filterType != "income" && filterType != "expense" {
			return nil, 0, fmt.Errorf("invalid type filter: must be 'income' or 'expense'")
		}
		conditions = append(conditions, fmt.Sprintf("type = $%d", len(args)+1))
		args = append(args, filterType)
	}

	if filterCategoryID > 0 {
		// Проверяем, существует ли категория и принадлежит ли она пользователю
		var exists bool
		err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1 AND user_id = $2)", filterCategoryID, userID).Scan(&exists)
		if err != nil {
			return nil, 0, err
		}
		if !exists {
			return nil, 0, fmt.Errorf("category does not exist or does not belong to user")
		}
		conditions = append(conditions, fmt.Sprintf("category_id = $%d", len(args)+1))
		args = append(args, filterCategoryID)
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
		countQuery += " AND " + strings.Join(conditions, " AND ")
	}

	var total int
	err := s.DB.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Запрос транзакций с пагинацией
	query := "SELECT id, user_id, amount, type, category_id, date FROM transactions WHERE user_id = $1"
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	if sort == "asc" || sort == "desc" {
		query += fmt.Sprintf(" ORDER BY date %s", sort)
	} else if sort != "" {
		return nil, 0, fmt.Errorf("invalid sort parameter: must be 'asc' or 'desc'")
	}

	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, (page-1)*limit)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}

	var transactions = []models.Transaction{}
	for rows.Next() {
		var t models.Transaction
		var categoryID sql.NullInt32
		err := rows.Scan(&t.ID, &t.UserID, &t.Amount, &t.Type, &categoryID, &t.Date)
		if err != nil {
			return nil, 0, err
		}
		if categoryID.Valid {
			t.CategoryID = int(categoryID.Int32)
		}
		transactions = append(transactions, t)
	}
	return transactions, total, nil
}

func (s *Storage) GetTransaction(id, userID int) (*models.Transaction, error) {
	var t models.Transaction
	var categoryID sql.NullInt32
	row := s.DB.QueryRow("SELECT id, user_id, amount, type, category_id, date FROM transactions WHERE id = $1 AND user_id = $2", id, userID)
	err := row.Scan(&t.ID, &t.UserID, &t.Amount, &t.Type, &categoryID, &t.Date)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if categoryID.Valid {
		t.CategoryID = int(categoryID.Int32)
	}
	return &t, nil
}

func (s *Storage) CreateTransaction(t *models.Transaction) error {
	if t.UserID == 0 {
		return fmt.Errorf("user_id is required")
	}
	if t.CategoryID <= 0 {
		return fmt.Errorf("category_id is required and must be positive")
	}

	var exists bool
	err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1 AND user_id = $2)", t.CategoryID, t.UserID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("category does not exist or does not belong to user")
	}

	if t.Date.IsZero() {
		t.Date = time.Now()
	}
	return s.DB.QueryRow("INSERT INTO transactions (user_id, amount, type, category_id, date) VALUES ($1, $2, $3, $4, $5) RETURNING id",
		t.UserID, t.Amount, t.Type, t.CategoryID, t.Date).
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

	if t.CategoryID > 0 {
		var exists bool
		err := s.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1 AND user_id = $2)", t.CategoryID, t.UserID).Scan(&exists)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, fmt.Errorf("category does not exist or does not belong to user")
		}
	}

	result, err := s.DB.Exec("UPDATE transactions SET amount = $1, type = $2, category_id = $3, date = $4 WHERE id = $5 AND user_id = $6",
		t.Amount, t.Type, t.CategoryID, t.Date, t.ID, t.UserID)

	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rowsAffected > 0, nil
}
