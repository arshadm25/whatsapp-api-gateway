package api

import (
	"io"
	"net/http"
	"whatsapp-gateway/internal/whatsapp"

	"github.com/gin-gonic/gin"
)

type WhatsAppHandler struct {
	Client *whatsapp.Client
}

func NewWhatsAppHandler(client *whatsapp.Client) *WhatsAppHandler {
	return &WhatsAppHandler{Client: client}
}

// SendMessage handles unified message sending
func (h *WhatsAppHandler) SendMessage(c *gin.Context) {
	var msg whatsapp.GenericMessage
	if err := c.ShouldBindJSON(&msg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure messaging_product is set
	if msg.MessagingProduct == "" {
		msg.MessagingProduct = "whatsapp"
	}

	if err := h.Client.SendRawMessage(msg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Message sent"})
}

// UploadMedia handles media file uploads
func (h *WhatsAppHandler) UploadMedia(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	mimeType := header.Header.Get("Content-Type")

	resp, err := h.Client.UploadMedia(fileBytes, mimeType, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// RetrieveMediaURL gets the URL for a media ID
func (h *WhatsAppHandler) RetrieveMediaURL(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Media ID required"})
		return
	}

	url, err := h.Client.RetrieveMediaURL(mediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

// DeleteMedia deletes a media object
func (h *WhatsAppHandler) DeleteMedia(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Media ID required"})
		return
	}

	if err := h.Client.DeleteMedia(mediaID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Media deleted"})
}

// GetTemplates retrieves templates from Meta
func (h *WhatsAppHandler) GetTemplates(c *gin.Context) {
	templates, err := h.Client.GetTemplates()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, templates)
}

// CreateTemplate creates a new template
func (h *WhatsAppHandler) CreateTemplate(c *gin.Context) {
	var templateData interface{}
	if err := c.ShouldBindJSON(&templateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.Client.CreateTemplate(templateData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteTemplate deletes a template by name
func (h *WhatsAppHandler) DeleteTemplate(c *gin.Context) {
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Template name required (query param 'name')"})
		return
	}

	if err := h.Client.DeleteTemplate(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Template deleted"})
}

// --- Flow Handlers ---

// GetFlows lists all flows
func (h *WhatsAppHandler) GetFlows(c *gin.Context) {
	flows, err := h.Client.GetFlows()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, flows)
}

// GetFlow gets a specific flow
func (h *WhatsAppHandler) GetFlow(c *gin.Context) {
	flowID := c.Param("id")
	flow, err := h.Client.GetFlow(flowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, flow)
}

// CreateFlow creates a new flow
func (h *WhatsAppHandler) CreateFlow(c *gin.Context) {
	var req struct {
		Name        string   `json:"name" binding:"required"`
		Categories  []string `json:"categories" binding:"required"`
		CloneFlowID string   `json:"clone_flow_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.Client.CreateFlow(req.Name, req.Categories, req.CloneFlowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateFlowMetadata updates flow name or categories
func (h *WhatsAppHandler) UpdateFlowMetadata(c *gin.Context) {
	flowID := c.Param("id")
	var req struct {
		Name       string   `json:"name"`
		Categories []string `json:"categories"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.Client.UpdateFlowMetadata(flowID, req.Name, req.Categories)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UploadFlowJSON uploads the JSON definition for a flow
func (h *WhatsAppHandler) UploadFlowJSON(c *gin.Context) {
	flowID := c.Param("id")
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File is required"})
		return
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	resp, err := h.Client.UploadFlowJSON(flowID, fileBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PublishFlow publishes a flow
func (h *WhatsAppHandler) PublishFlow(c *gin.Context) {
	flowID := c.Param("id")
	resp, err := h.Client.PublishFlow(flowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteFlow deletes a flow
func (h *WhatsAppHandler) DeleteFlow(c *gin.Context) {
	flowID := c.Param("id")
	resp, err := h.Client.DeleteFlow(flowID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
