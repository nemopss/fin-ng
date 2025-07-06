package api

import (
	"net/http"
	"strconv"

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

func (h *Handler) GetTransactions(c *gin.Context) {
	transactions, err := h.storage.GetTransactions()
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
