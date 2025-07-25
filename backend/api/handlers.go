package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/nemopss/fin-ng/backend/db"
	"github.com/nemopss/fin-ng/backend/models"
	"golang.org/x/crypto/bcrypt"
)

// FIX: swagger output models

type Handler struct {
	storage   *db.Storage
	jwtSecret string
}

func NewHandler(s *db.Storage, jwtSecret string) *Handler {
	return &Handler{storage: s, jwtSecret: jwtSecret}
}

func validateTransaction(t models.Transaction) error {
	if t.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if t.Type != "income" && t.Type != "expense" {
		return fmt.Errorf("type must be 'income' or 'expense'")
	}
	if t.CategoryID <= 0 {
		return fmt.Errorf("category_id is required and must be positive")
	}
	return nil
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := c.GetHeader("Authorization")
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(h.jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token claims"})
			c.Abort()
			return
		}

		userID, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user_id in token"})
			c.Abort()
			return
		}

		c.Set("user_id", int(userID))
		c.Next()
	}
}

// @Summary Регистрация нового пользователя
// @Description Создает нового пользователя с именем пользователя и паролем
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body models.CreateUser true "Данные пользователя"
// @Success 201 {object} models.RegisterResponse"
// @Failure 400 {object} models.ErrorResponse
// @Router /register [post]
func (h *Handler) Register(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(user.Password) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 6 characters"})
		return
	}

	createdUser, err := h.storage.CreateUser(user.Username, user.Password)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": createdUser.ID, "username": createdUser.Username})
}

// @Summary Вход пользователя
// @Description Аутентифицирует пользователя и возвращает JWT токен
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body models.CreateUser true "Данные пользователя"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /login [post]
func (h *Handler) Login(c *gin.Context) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.storage.GetUserByUsername(credentials.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(credentials.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// @Security ApiKeyAuth
// @Summary Создать новую категорию
// @Description Создает новую категорию для пользователя
// @Tags categories
// @Accept json
// @Produce json
// @Param category body models.CreateCategory true "Данные категории"
// @Success 201 {object} models.Category
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /categories [post]
func (h *Handler) CreateCategory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if category.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category name is required"})
		return
	}

	createdCategory, err := h.storage.CreateCategory(userID.(int), category.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createdCategory)
}

// @Security ApiKeyAuth
// @Summary Получить список категорий
// @Description Получает список категорий пользователя
// @Tags categories
// @Produce json
// @Success 200 {array} models.Category
// @Failure 401 {object} models.ErrorResponse
// @Router /categories [get]
func (h *Handler) GetCategories(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}
	categories, err := h.storage.GetCategories(userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// @Security ApiKeyAuth
// @Summary Получить категорию
// @Description Получает категорию пользователя по ID
// @Tags categories
// @Produce json
// @Param id path int true "ID категории"
// @Success 200 {object} models.Category
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object}  models.ErrorResponse
// @Router /categories/{id} [get]
func (h *Handler) GetCategory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
		return
	}
	category, err := h.storage.GetCategory(id, userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if category == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}

	c.JSON(http.StatusOK, category)
}

// @Security ApiKeyAuth
// @Summary Обновить категорию
// @Description Обновляет существующую категорию пользователя
// @Tags categories
// @Accept json
// @Produce json
// @Param id path int true "ID категории"
// @Param category body models.CreateCategory true "Новое имя категории"
// @Success 200 {object} models.UpdateCategoryResponse"
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /categories/{id} [put]
func (h *Handler) UpdateCategory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
		return
	}

	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if category.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category name is required"})
		return
	}

	updated, err := h.storage.UpdateCategory(id, userID.(int), category.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !updated {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id, "user_id": userID, "name": category.Name})
}

// @Security ApiKeyAuth
// @Summary Удалить категорию
// @Description Удаляет категорию пользователя, если она не используется в транзакциях
// @Tags categories
// @Produce json
// @Param id path int true "ID категории"
// @Success 204
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /categories/{id} [delete]
func (h *Handler) DeleteCategory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category id"})
		return
	}

	deleted, err := h.storage.DeleteCategory(id, userID.(int))
	if err != nil {
		if strings.Contains(err.Error(), "category is used in transactions") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category is used in transactions"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	if !deleted {
		c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Security ApiKeyAuth
// @Summary Получить список транзакций
// @Description Получает список транзакций пользователя с возможностью фильтрации и пагинации
// @Tags transactions
// @Produce json
// @Param type query string false "Тип транзакции (income или expense)"
// @Param category_id query int false "ID категории"
// @Param min_amount query number false "Минимальная сумма"
// @Param max_amount query number false "Максимальная сумма"
// @Param sort query string false "Сортировка по дате (asc или desc)"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Лимит на страницу"
// @Success 200 {object} models.GetTransactionsResponse"
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /transactions [get]
func (h *Handler) GetTransactions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	filterType := c.Query("type")
	filterCategoryIDStr := c.Query("category_id")
	minAmountStr := c.Query("min_amount")
	maxAmountStr := c.Query("max_amount")
	sort := c.Query("sort")
	pageStr := c.Query("page")
	limitStr := c.Query("limit")

	var filterCategoryID int
	var minAmount, maxAmount float64
	var page, limit int
	var err error

	if filterCategoryIDStr != "" {
		filterCategoryID, err = strconv.Atoi(filterCategoryIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
			return
		}
		if filterCategoryID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category_id must be positive"})
			return
		}
		category, err := h.storage.GetCategory(filterCategoryID, userID.(int))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if category == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category does not exist or does not belong to user"})
			return
		}
	}

	if minAmountStr != "" {
		minAmount, err = strconv.ParseFloat(minAmountStr, 64)
		if err != nil || minAmount < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid min_amount"})
			return
		}
	}

	if maxAmountStr != "" {
		maxAmount, err = strconv.ParseFloat(maxAmountStr, 64)
		if err != nil || maxAmount < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid max_amount"})
			return
		}
	}

	if filterType != "" && filterType != "income" && filterType != "expense" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'income' or 'expense'"})
		return
	}

	if sort != "" && sort != "asc" && sort != "desc" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sort must be 'asc' or 'desc'"})
		return
	}

	if pageStr == "" {
		page = 1
	} else {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "page must be a positive integer"})
			return
		}
	}

	if limitStr == "" {
		limit = 10
	} else {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 100"})
			return
		}
	}

	transactions, total, err := h.storage.GetTransactions(userID.(int), filterType, filterCategoryID, minAmount, maxAmount, sort, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"total":        total,
	})
}

// @Security ApiKeyAuth
// @Summary Получить транзакцию по ID
// @Description Получает детали конкретной транзакции пользователя
// @Tags transactions
// @Produce json
// @Param id path int true "ID транзакции"
// @Success 200 {object} models.Transaction
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /transactions/{id} [get]
func (h *Handler) GetTransaction(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transaction, err := h.storage.GetTransaction(id, userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error in get transaction": err.Error()})
		return
	}
	if transaction == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}
	c.JSON(http.StatusOK, transaction)
}

// @Security ApiKeyAuth
// @Summary Создать новую транзакцию
// @Description Создает новую транзакцию для пользователя
// @Tags transactions
// @Accept json
// @Produce json
// @Param transaction body models.CreateTransaction true "Данные транзакции"
// @Success 201 {object} models.Transaction
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Router /transactions [post]
func (h *Handler) CreateTransaction(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	var newTransaction = models.Transaction{}
	if err := c.ShouldBindJSON(&newTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := validateTransaction(newTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newTransaction.UserID = userID.(int)
	if newTransaction.Date.IsZero() {
		newTransaction.Date = time.Now()
	}

	if err := h.storage.CreateTransaction(&newTransaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, newTransaction)

}

// @Security ApiKeyAuth
// @Summary Удалить транзакцию
// @Description Удаляет транзакцию пользователя
// @Tags transactions
// @Produce json
// @Param id path int true "ID транзакции"
// @Success 204
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /transactions/{id} [delete]
func (h *Handler) DeleteTransaction(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ok, err := h.storage.DeleteTransaction(id, userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ok == false {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Security ApiKeyAuth
// @Summary Обновить транзакцию
// @Description Обновляет существующую транзакцию пользователя
// @Tags transactions
// @Accept json
// @Produce json
// @Param id path int true "ID транзакции"
// @Param transaction body models.CreateTransaction true "Новые данные транзакции"
// @Success 200 {object} models.Transaction
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /transactions/{id} [put]
func (h *Handler) UpdateTransaction(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transaction, err := h.storage.GetTransaction(id, userID.(int))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if transaction == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	var updatedTransaction models.Transaction
	if err := c.ShouldBindJSON(&updatedTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedTransaction.ID = id
	updatedTransaction.UserID = userID.(int)

	if err := validateTransaction(updatedTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if updatedTransaction.Date.IsZero() {
		updatedTransaction.Date = time.Now()
	}

	ok, err := h.storage.UpdateTransaction(&updatedTransaction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if ok == false {
		c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		return
	}

	c.JSON(http.StatusOK, updatedTransaction)
}
