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

	// basic processing
	if len(payload.Entry) > 0 && len(payload.Entry[0].Changes) > 0 {
		value := payload.Entry[0].Changes[0].Value
		if len(value.Messages) > 0 {
			message := value.Messages[0]

			// Determine content and type based on message type
			var content string
			msgType := message.Type

			switch message.Type {
			case "text":
				content = message.Text.Body
				log.Printf("Received text message from %s: %s", message.From, content)
			case "image":
				if message.Image != nil {
					content = "[image]:" + message.Image.ID
					if message.Image.Caption != "" {
						content += ":" + message.Image.Caption
					}
				}
				log.Printf("Received image from %s: %s", message.From, content)
			case "video":
				if message.Video != nil {
					content = "[video]:" + message.Video.ID
					if message.Video.Caption != "" {
						content += ":" + message.Video.Caption
					}
				}
				log.Printf("Received video from %s", message.From)
			case "audio":
				if message.Audio != nil {
					content = "[audio]:" + message.Audio.ID
				}
				log.Printf("Received audio from %s", message.From)
			case "document":
				if message.Document != nil {
					content = "[document]:" + message.Document.ID
					if message.Document.Filename != "" {
						content += ":" + message.Document.Filename
					}
				}
				log.Printf("Received document from %s", message.From)
			default:
				content = "[" + message.Type + "]"
				log.Printf("Received %s from %s", message.Type, message.From)
			}

			// Store message in DB
			stmt, err := database.DB.Prepare("INSERT INTO messages(wa_id, sender, content, type, status) VALUES(?, ?, ?, ?, ?)")
			if err != nil {
				log.Printf("Error preparing db statement: %v", err)
			} else {
				_, err = stmt.Exec(message.ID, message.From, content, msgType, "received")
				if err != nil {
					log.Printf("Error inserting into db: %v", err)
				}
			}

			// Auto-save Contact
			userName := message.From
			_, err = database.DB.Exec(`INSERT INTO contacts(wa_id, name, tags) VALUES(?, ?, '[]') 
				ON CONFLICT(wa_id) DO UPDATE SET name=excluded.name WHERE name IS NULL OR name = ''`, message.From, userName)
			if err != nil {
				log.Printf("Error saving contact: %v", err)
			}

			// Process through automation engine (only for text messages)
			if h.AutomationEngine != nil && message.Type == "text" {
				go h.AutomationEngine.ProcessIncomingMessage(message.From, message.Text.Body)
			}
		}
	}

	c.Status(http.StatusOK)
}
