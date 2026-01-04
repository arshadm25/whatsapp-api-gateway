package api

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"time"
	"whatsapp-gateway/internal/database"
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

	// Get MIME type from header, but fallback to extension if it's generic
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		// Detect from file extension
		ext := filepath.Ext(header.Filename)
		if detectedType := mime.TypeByExtension(ext); detectedType != "" {
			mimeType = detectedType
		}
	}

	resp, err := h.Client.UploadMedia(fileBytes, mimeType, header.Filename)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Save to database for persistence
	_, err = database.DB.Exec(`
		INSERT INTO media (media_id, filename, mime_type, file_size)
		VALUES (?, ?, ?, ?)
	`, resp.ID, header.Filename, mimeType, header.Size)
	if err != nil {
		// Log but don't fail - upload to WhatsApp succeeded
		c.JSON(http.StatusOK, gin.H{
			"id":       resp.ID,
			"filename": header.Filename,
			"warning":  "Upload succeeded but failed to save to local database",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        resp.ID,
		"filename":  header.Filename,
		"mime_type": mimeType,
		"file_size": header.Size,
	})
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

// DownloadMediaProxy downloads media from WhatsApp and serves it (as a proxy)
func (h *WhatsAppHandler) DownloadMediaProxy(c *gin.Context) {
	mediaID := c.Param("id")
	if mediaID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Media ID required"})
		return
	}

	// Get the media URL from WhatsApp
	mediaURL, err := h.Client.RetrieveMediaURL(mediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Download the media with authentication
	req, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+h.Client.Config.WhatsAppToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	// Set the content type from the response
	contentType := resp.Header.Get("Content-Type")
	if contentType != "" {
		c.Header("Content-Type", contentType)
	}

	// Stream the response body to the client
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
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

	// Also remove from local database
	database.DB.Exec("DELETE FROM media WHERE media_id = ?", mediaID)

	c.JSON(http.StatusOK, gin.H{"status": "Media deleted"})
}

// ListMedia lists all stored media
func (h *WhatsAppHandler) ListMedia(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT id, media_id, filename, mime_type, file_size, uploaded_at
		FROM media
		ORDER BY uploaded_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var mediaList []gin.H
	for rows.Next() {
		var id int
		var mediaID, filename, mimeType string
		var fileSize int64
		var uploadedAt string

		if err := rows.Scan(&id, &mediaID, &filename, &mimeType, &fileSize, &uploadedAt); err != nil {
			continue
		}

		mediaList = append(mediaList, gin.H{
			"id":          id,
			"media_id":    mediaID,
			"filename":    filename,
			"mime_type":   mimeType,
			"file_size":   fileSize,
			"uploaded_at": uploadedAt,
		})
	}

	c.JSON(http.StatusOK, mediaList)
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

	// Fetch graph_data from local DB
	var graphData string
	err = database.DB.QueryRow("SELECT graph_data FROM flows WHERE id = ?", flowID).Scan(&graphData)
	if err == nil && graphData != "" {
		// If flow is a map, map it
		if flowMap, ok := flow.(map[string]interface{}); ok {
			// We need to unmarshal graphData string back to object to nest it properly
			var graphObj interface{}
			if err := json.Unmarshal([]byte(graphData), &graphObj); err == nil {
				flowMap["graph_data"] = graphObj
			} else {
				flowMap["graph_data"] = graphData // fallback to string
			}
			c.JSON(http.StatusOK, flowMap)
			return
		}
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

	// Save graph_data to local DB
	graphData := c.PostForm("graph_data")
	if graphData != "" {
		_, err := database.DB.Exec(`INSERT INTO flows (id, graph_data) VALUES (?, ?) 
			ON CONFLICT(id) DO UPDATE SET graph_data=excluded.graph_data, updated_at=CURRENT_TIMESTAMP`, flowID, graphData)
		if err != nil {
			// Log error but don't fail the request since Meta upload succeeded
			fmt.Printf("Error saving graph data: %v\n", err)
		}
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

// --- Local Flow Storage ---

// SaveLocalFlow saves a flow to local DB
func (h *WhatsAppHandler) SaveLocalFlow(c *gin.Context) {
	var req struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		GraphData string `json:"graph_data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ID == "" {
		req.ID = fmt.Sprintf("flow_%d", time.Now().Unix())
	}

	_, err := database.DB.Exec(`
		INSERT INTO flows (id, name, status, graph_data) 
		VALUES (?, ?, 'draft', ?)
		ON CONFLICT(id) DO UPDATE SET 
			name=excluded.name, 
			graph_data=excluded.graph_data, 
			updated_at=CURRENT_TIMESTAMP
	`, req.ID, req.Name, req.GraphData)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": req.ID, "status": "saved"})
}

// GetLocalFlows lists local flows
func (h *WhatsAppHandler) GetLocalFlows(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, updated_at FROM flows ORDER BY updated_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var flows []gin.H
	for rows.Next() {
		var id, name, updatedAt string
		if err := rows.Scan(&id, &name, &updatedAt); err != nil {
			continue
		}
		flows = append(flows, gin.H{"id": id, "name": name, "updated_at": updatedAt})
	}
	if flows == nil {
		flows = []gin.H{}
	}
	c.JSON(http.StatusOK, flows)
}

// GetLocalFlow gets a single local flow
func (h *WhatsAppHandler) GetLocalFlow(c *gin.Context) {
	id := c.Param("id")
	var name, graphData string
	err := database.DB.QueryRow("SELECT name, graph_data FROM flows WHERE id = ?", id).Scan(&name, &graphData)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Flow not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         id,
		"name":       name,
		"graph_data": json.RawMessage(graphData),
	})
}
