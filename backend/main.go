package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type Transaction struct {
	ID     int     `json:"id"`
	Amount float64 `json:"amount"`
	Type   string  `json:"type"`
}

var transactions = []Transaction{}

func main() {
	r := gin.Default()

	r.GET("/transactions", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, transactions)
	})

	r.POST("/transactions", func(ctx *gin.Context) {
		var newTransaction Transaction
		if err := ctx.ShouldBindJSON(&newTransaction); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		transactions = append(transactions, newTransaction)
		ctx.JSON(http.StatusCreated, newTransaction)
	})

	r.Run()
}
