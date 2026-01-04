package webhook

import (
	"log"
	"net/http"
	"whatsapp-gateway/internal/automation"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"
	pkgModels "whatsapp-gateway/pkg/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	var payload pkgModels.WebhookPayload
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
			case "interactive":
				if message.Interactive != nil {
					if message.Interactive.Type == "button_reply" && message.Interactive.ButtonReply != nil {
						// User clicked a button - use the button title as the message content
						content = message.Interactive.ButtonReply.Title
						log.Printf("Received button click from %s: %s (ID: %s)", message.From, content, message.Interactive.ButtonReply.ID)
					} else if message.Interactive.Type == "list_reply" && message.Interactive.ListReply != nil {
						// User selected from a list
						content = message.Interactive.ListReply.Title
						log.Printf("Received list selection from %s: %s", message.From, content)
					} else if message.Interactive.Type == "nfm_reply" && message.Interactive.NfmReply != nil {
						// This is a Flow response
						reply := message.Interactive.NfmReply
						content = "[flow_response]:" + reply.ResponsePayload
						log.Printf("Received Flow response from %s: %s", message.From, reply.ResponsePayload)
					} else {
						content = "[interactive]:" + message.Interactive.Type
					}
				}
				log.Printf("Received interactive message from %s", message.From)
			default:
				content = "[" + message.Type + "]"
				log.Printf("Received %s from %s", message.Type, message.From)
			}

			// Store message in DB
			msgModel := models.Message{
				WaID:    message.ID,
				Sender:  message.From,
				Content: content,
				Type:    msgType,
				Status:  "received",
			}
			if err := database.GormDB.Create(&msgModel).Error; err != nil {
				log.Printf("Error inserting into db: %v", err)
			}

			// Auto-save Contact
			var contact models.Contact
			err := database.GormDB.Where("wa_id = ?", message.From).First(&contact).Error
			if err == gorm.ErrRecordNotFound {
				contact = models.Contact{
					WaID: message.From,
					Name: message.From, // Default to phone number
					Tags: "[]",
				}
				database.GormDB.Create(&contact)
			} else if err == nil {
				if contact.Name == "" || contact.Name == contact.WaID {
					// Update name if currently empty or just the phone number
					database.GormDB.Model(&contact).Update("name", message.From)
				}
			}

			// Process through automation engine (text and interactive messages)
			if h.AutomationEngine != nil {
				// Determine the message content to process
				var messageContent string
				if message.Type == "text" {
					messageContent = message.Text.Body
				} else if message.Type == "interactive" && content != "" {
					// For interactive messages, use the extracted content (button title, list selection, etc.)
					messageContent = content
				}

				// Process if we have content
				if messageContent != "" {
					go h.AutomationEngine.ProcessIncomingMessage(message.From, messageContent)
				}
			}
		}
	}

	c.Status(http.StatusOK)
}
