package db

import (
	"database/sql"

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
		type TEXT
	)`)

	if err != nil {
		return nil, err
	}

	return &Storage{DB: db}, nil
}

func (s *Storage) Close() {
	s.DB.Close()
}

func (s *Storage) GetTransactions() ([]models.Transaction, error) {
	rows, err := s.DB.Query("SELECT id, amount, type FROM transactions")
	if err != nil {
		return nil, err
	}

	var transactions = []models.Transaction{}
	for rows.Next() {
		var t models.Transaction
		err := rows.Scan(&t.ID, &t.Amount, &t.Type)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func (s *Storage) CreateTransaction(t *models.Transaction) error {
	return s.DB.QueryRow("INSERT INTO transactions (amount, type) VALUES ($1, $2) RETURNING id", t.Amount, t.Type).Scan(&t.ID)
}
