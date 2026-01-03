package api

import (
	"database/sql"
	"encoding/json"
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

// SyncTemplates fetches templates from Meta and stores them locally
func (h *BroadcastHandler) SyncTemplates(c *gin.Context) {
	if h.Config.WhatsAppBusinessAccountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "WABA_ID not configured in .env"})
		return
	}

	// Fetch from Meta API
	rawTemplates, err := h.Client.GetTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch templates from Meta: " + err.Error()})
		return
	}

	// Parse the response
	templatesMap, ok := rawTemplates.(map[string]interface{})
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid response format from Meta"})
		return
	}

	data, ok := templatesMap["data"].([]interface{})
	if !ok {
		c.JSON(http.StatusOK, gin.H{"status": "No templates found", "count": 0})
		return
	}

	// Store templates in database
	syncedCount := 0
	for _, item := range data {
		tmpl, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		id := tmpl["id"].(string)
		name := tmpl["name"].(string)
		language := ""
		if lang, ok := tmpl["language"].(string); ok {
			language = lang
		}
		category := ""
		if cat, ok := tmpl["category"].(string); ok {
			category = cat
		}
		status := ""
		if st, ok := tmpl["status"].(string); ok {
			status = st
		}

		// Serialize components to JSON string
		componentsJSON := "[]"
		if components, ok := tmpl["components"]; ok {
			compBytes, err := json.Marshal(components)
			if err == nil {
				componentsJSON = string(compBytes)
			}
		}

		// Upsert into database
		_, err = database.DB.Exec(`INSERT INTO templates(id, name, language, category, status, components) 
			VALUES(?, ?, ?, ?, ?, ?) 
			ON CONFLICT(id) DO UPDATE SET name=excluded.name, language=excluded.language, 
			category=excluded.category, status=excluded.status, components=excluded.components`,
			id, name, language, category, status, componentsJSON)
		if err != nil {
			log.Printf("Error saving template %s: %v", name, err)
			continue
		}
		syncedCount++
	}

	c.JSON(http.StatusOK, gin.H{"status": "Templates synced", "count": syncedCount})
}

// GetTemplatesFromMeta returns raw templates from Meta API (not cached)
func (h *BroadcastHandler) GetTemplatesFromMeta(c *gin.Context) {
	if h.Config.WhatsAppBusinessAccountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "WABA_ID not configured"})
		return
	}

	templates, err := h.Client.GetTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, templates)
}

// GetTemplates returns stored templates from local database
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
		var components sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.Language, &t.Category, &t.Status, &components); err != nil {
			log.Printf("Error scanning template: %v", err)
			continue
		}
		if components.Valid {
			t.Components = components.String
		}
		templates = append(templates, t)
	}

	// Return empty array instead of null
	if templates == nil {
		templates = []models.Template{}
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
