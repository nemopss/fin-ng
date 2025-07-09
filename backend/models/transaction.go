package models

import "time"

type Transaction struct {
	ID         int       `json:"id"`
	UserID     int       `json:"user_id"`
	Amount     float64   `json:"amount"`
	Type       string    `json:"type"`
	CategoryID int       `json:"category_id"`
	Date       time.Time `json:"date"`
}
