package models

import "time"

type Transaction struct {
	ID     int       `json:"id"`
	Amount float64   `json:"amount"`
	Type   string    `json:"type"`
	Date   time.Time `json:"date"`
}
