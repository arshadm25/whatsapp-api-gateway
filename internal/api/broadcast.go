package api

import (
	"log"
	"net/http"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/whatsapp"
	"whatsapp-gateway/pkg/models"

	"github.com/gin-gonic/gin"
)

type BroadcastHandler struct {
	Client *whatsapp.Client
	Config *config.Config
}

func NewBroadcastHandler(client *whatsapp.Client, cfg *config.Config) *BroadcastHandler {
	return &BroadcastHandler{Client: client, Config: cfg}
}

// SyncTemplates fetches templates from Meta and stores them
func (h *BroadcastHandler) SyncTemplates(c *gin.Context) {
	if h.Config.WhatsAppBusinessAccountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "WABA_ID not configured in .env"})
		return
	}

	// Fetch from Meta (Mock implementation or Real call would go here)
	// Real call: https://graph.facebook.com/v19.0/{WABA_ID}/message_templates
	// For now, we will assume we fetch them and insert dummy/real data

	// TODO: Implement actual HTTP call to Meta.
	// For the sake of this demo, let's allow creating a template manually or mock it.

	c.JSON(http.StatusOK, gin.H{"status": "Templates synced (stub)"})
}

// GetTemplates returns stored templates
func (h *BroadcastHandler) GetTemplates(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, language, category, status, components FROM templates")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var templates []models.Template
	for rows.Next() {
		var t models.Template
		if err := rows.Scan(&t.ID, &t.Name, &t.Language, &t.Category, &t.Status, &t.Components); err != nil {
			log.Printf("Error scanning template: %v", err)
			continue
		}
		templates = append(templates, t)
	}
	c.JSON(http.StatusOK, templates)
}

type BroadcastRequest struct {
	TemplateName string   `json:"template_name"`
	Language     string   `json:"language"`
	Contacts     []string `json:"contacts"` // List of WA IDs
}

func (h *BroadcastHandler) SendBroadcast(c *gin.Context) {
	var req BroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Iterate and send (in a real app, use a queue)
	successCount := 0
	for _, waID := range req.Contacts {
		// logic to send template message via Client
		err := h.Client.SendTemplateMessage(waID, req.TemplateName, req.Language)
		if err == nil {
			successCount++
		} else {
			log.Printf("Failed to broadcast to %s: %v", waID, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "Broadcast processed",
		"sent_to": successCount,
		"total":   len(req.Contacts),
	})
}
