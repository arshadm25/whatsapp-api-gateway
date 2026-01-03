package webhook

import (
	"log"
	"net/http"
	"whatsapp-gateway/internal/automation"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/pkg/models"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Config           *config.Config
	AutomationEngine *automation.Engine
}

func NewHandler(cfg *config.Config, automationEngine *automation.Engine) *Handler {
	return &Handler{
		Config:           cfg,
		AutomationEngine: automationEngine,
	}
}

func (h *Handler) VerifyWebhook(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")

	if mode != "" && token != "" {
		if mode == "subscribe" && token == h.Config.VerifyToken {
			log.Println("Webhook verified successfully!")
			c.String(http.StatusOK, challenge)
		} else {
			c.Status(http.StatusForbidden)
		}
	} else {
		c.Status(http.StatusBadRequest)
	}
}

func (h *Handler) HandleMessage(c *gin.Context) {
	var payload models.WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		log.Printf("Error binding JSON: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	// Simple logging of the payload
	// log.Printf("Received payload: %+v", payload)

	// basic processing
	if len(payload.Entry) > 0 && len(payload.Entry[0].Changes) > 0 {
		value := payload.Entry[0].Changes[0].Value
		if len(value.Messages) > 0 {
			message := value.Messages[0]
			log.Printf("Received message from %s: %s", message.From, message.Text.Body)

			// Store message in DB
			stmt, err := database.DB.Prepare("INSERT INTO messages(wa_id, sender, content, type, status) VALUES(?, ?, ?, ?, ?)")
			if err != nil {
				log.Printf("Error preparing db statement: %v", err)
			} else {
				_, err = stmt.Exec(message.ID, message.From, message.Text.Body, message.Type, "received")
				if err != nil {
					log.Printf("Error inserting into db: %v", err)
				}
			}

			// Auto-save Contact
			// If name is not provided in payload, use the phone number or existing name
			userName := message.From
			// In real payloads, contacts array often has the profile name.
			// For now, we just ensure the contact exists.

			// Upsert Contact (Simple SQLite UPSERT)
			_, err = database.DB.Exec(`INSERT INTO contacts(wa_id, name, tags) VALUES(?, ?, '[]') 
				ON CONFLICT(wa_id) DO UPDATE SET name=excluded.name WHERE name IS NULL OR name = ''`, message.From, userName)
			if err != nil {
				log.Printf("Error saving contact: %v", err)
			}

			// Process through automation engine
			if h.AutomationEngine != nil {
				go h.AutomationEngine.ProcessIncomingMessage(message.From, message.Text.Body)
			}
		}
	}

	c.Status(http.StatusOK)
}
