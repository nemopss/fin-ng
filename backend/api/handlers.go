package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nemopss/fin-ng/backend/db"
	"github.com/nemopss/fin-ng/backend/models"
)

type Handler struct {
	storage *db.Storage
}

func NewHandler(s *db.Storage) *Handler {
	return &Handler{storage: s}
}

func validateTransaction(t models.Transaction) error {
	if t.Amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if t.Type != "income" && t.Type != "expense" {
		return fmt.Errorf("type must be 'income' or 'expense'")
	}
	return nil
}

func (h *Handler) GetTransactions(c *gin.Context) {
	filterType := c.Query("type")
	minAmountStr := c.Query("min_amount")
	maxAmountStr := c.Query("max_amount")
	sort := c.Query("sort")

	var minAmount, maxAmount float64
	var err error

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

	transactions, err := h.storage.GetTransactions(filterType, minAmount, maxAmount, sort)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, transactions)
}

func (h *Handler) GetTransaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	transaction, err := h.storage.GetTransaction(id)
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

func (h *Handler) CreateTransaction(c *gin.Context) {
	var newTransaction = models.Transaction{}
	if err := c.ShouldBindJSON(&newTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := validateTransaction(newTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if newTransaction.Date.IsZero() {
		newTransaction.Date = time.Now()
	}

	if err := h.storage.CreateTransaction(&newTransaction); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, newTransaction)

}

func (h *Handler) DeleteTransaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ok, err := h.storage.DeleteTransaction(id)
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

func (h *Handler) UpdateTransaction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var updatedTransaction models.Transaction
	if err := c.ShouldBindJSON(&updatedTransaction); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updatedTransaction.ID = id

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
