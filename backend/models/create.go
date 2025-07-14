package models

type CreateTransaction struct {
	Amount     float64 `json:"amount"`
	Type       string  `json:"type"`
	CaregoryID int     `json:"category_id"`
}

type CreateUser struct {
	Login    string `json:"username"`
	Password string `json:"password"`
}

type CreateCategory struct {
	Name string `json:"name"`
}
