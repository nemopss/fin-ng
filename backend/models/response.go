package models

type RegisterResponse struct {
	ID       int    `json:"id" example:"1"`
	Username string `json:"username" example:"john_doe"`
}

type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

type UpdateCategoryResponse struct {
	ID     int    `json:"id" example:"1"`
	UserID int    `json:"user_id" example:"1"`
	Name   string `json:"name" example:"Food"`
}

type GetTransactionsResponse struct {
	Transactions []Transaction `json:"transactions"`
	Total        int           `json:"total" example:"100"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"error"`
}
