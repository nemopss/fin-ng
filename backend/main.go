package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	//"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/nemopss/fin-ng/backend/api"
	"github.com/nemopss/fin-ng/backend/db"
	_ "github.com/nemopss/fin-ng/backend/docs"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
)

// @SecurityDefinitions.apikey ApiKeyAuth
// @In header
// @Name Authorization
func main() {
	/* if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	} */

	// Подключение к PostgreSQL
	connStr := os.Getenv("POSTGRES_URL")
	storage, err := db.NewStorage(connStr)
	if err != nil {
		panic(err)
	}
	defer storage.Close()

	// Получение JWT_SECRET из .env
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	handler := api.NewHandler(storage, jwtSecret)

	r := gin.Default()
	r.POST("/register", handler.Register)
	r.POST("/login", handler.Login)

	protected := r.Group("/", handler.AuthMiddleware())
	protected.GET("/transactions", handler.GetTransactions)
	protected.GET("/transactions/:id", handler.GetTransaction)
	protected.POST("/transactions", handler.CreateTransaction)
	protected.DELETE("/transactions/:id", handler.DeleteTransaction)
	protected.PUT("/transactions/:id", handler.UpdateTransaction)
	protected.POST("/categories", handler.CreateCategory)
	protected.GET("/categories", handler.GetCategories)
	protected.GET("/categories/:id", handler.GetCategory)
	protected.PUT("/categories/:id", handler.UpdateCategory)
	protected.DELETE("/categories/:id", handler.DeleteCategory)

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.Run()
}
