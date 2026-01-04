package api

import (
	"net/http"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"
	"whatsapp-gateway/internal/whatsapp"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	Client *whatsapp.Client
}

func NewDashboardHandler(client *whatsapp.Client) *DashboardHandler {
	return &DashboardHandler{Client: client}
}

func (h *DashboardHandler) GetMessages(c *gin.Context) {
	var messages []models.Message
	if err := database.GormDB.Order("created_at desc").Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

type SendRequest struct {
	To      string `json:"to"`
	Content string `json:"content"`
}

func (h *DashboardHandler) SendMessage(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Client.SendMessage(req.To, req.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Message sent"})
}
