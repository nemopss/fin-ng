package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nemopss/fin-ng/backend/api"
	"github.com/nemopss/fin-ng/backend/db"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Подключение к PostgreSQL
	connStr := os.Getenv("POSTGRES_URL")
	storage, err := db.NewStorage(connStr)
	if err != nil {
		panic(err)
	}
	defer storage.Close()

	handler := api.NewHandler(storage)

	r := gin.Default()
	r.GET("/transactions", handler.GetTransactions)
	r.GET("/transaction/:id", handler.GetTransaction)
	r.POST("/transactions", handler.CreateTransaction)
	r.DELETE("/transaction/:id", handler.DeleteTransaction)
	r.PUT("/transaction/:id", handler.UpdateTransaction)

	r.Run()
}
