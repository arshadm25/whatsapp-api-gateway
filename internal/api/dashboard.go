package api

import (
	"log"
	"net/http"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/whatsapp"
	"whatsapp-gateway/pkg/models"

	"github.com/gin-gonic/gin"
)

type DashboardHandler struct {
	Client *whatsapp.Client
}

func NewDashboardHandler(client *whatsapp.Client) *DashboardHandler {
	return &DashboardHandler{Client: client}
}

func (h *DashboardHandler) GetMessages(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, wa_id, sender, content, type, status, created_at FROM messages ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(&m.ID, &m.WaID, &m.Sender, &m.Content, &m.Type, &m.Status, &m.CreatedAt); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		messages = append(messages, m)
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
