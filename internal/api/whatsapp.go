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
	"whatsapp-gateway/internal/models"
	"whatsapp-gateway/internal/whatsapp"

	"whatsapp-gateway/internal/automation"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
	media := models.Media{
		MediaID:  resp.ID,
		Filename: header.Filename,
		MimeType: mimeType,
		FileSize: header.Size,
	}

	if err := database.GormDB.Create(&media).Error; err != nil {
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
	database.GormDB.Where("media_id = ?", mediaID).Delete(&models.Media{})

	c.JSON(http.StatusOK, gin.H{"status": "Media deleted"})
}

// ListMedia lists all stored media
func (h *WhatsAppHandler) ListMedia(c *gin.Context) {
	var mediaList []models.Media
	if err := database.GormDB.Order("uploaded_at DESC").Find(&mediaList).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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
	graphJSON, err := h.getFlowGraph(flowID)
	if err == nil && graphJSON != "" {
		// If flow is a map, map it
		if flowMap, ok := flow.(map[string]interface{}); ok {
			var graphObj interface{}
			if err := json.Unmarshal([]byte(graphJSON), &graphObj); err == nil {
				flowMap["graph_data"] = graphObj
			} else {
				flowMap["graph_data"] = graphJSON // fallback to string
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
		flow := models.Flow{
			ID:   flowID,
			Name: "Imported Flow " + flowID, // Default name if unknown
		}
		if err := database.GormDB.FirstOrCreate(&flow).Error; err == nil {
			if err := h.syncFlowGraph(flowID, graphData); err != nil {
				fmt.Printf("Error syncing graph data: %v\n", err)
			}
		} else {
			fmt.Printf("Error saving flow record: %v\n", err)
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

	flow := models.Flow{
		ID:     req.ID,
		Name:   req.Name,
		Status: "draft",
	}

	if err := database.GormDB.Save(&flow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.syncFlowGraph(req.ID, req.GraphData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Saved flow but failed to sync relational graph: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": req.ID, "status": "saved"})
}

func (h *WhatsAppHandler) syncFlowGraph(flowID string, graphData string) error {
	var graph automation.FlowGraphData
	if err := json.Unmarshal([]byte(graphData), &graph); err != nil {
		return err
	}

	return database.GormDB.Transaction(func(tx *gorm.DB) error {
		// Delete existing nodes and edges
		if err := tx.Where("flow_id = ?", flowID).Delete(&models.FlowNode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_id = ?", flowID).Delete(&models.FlowEdge{}).Error; err != nil {
			return err
		}

		// Insert new nodes
		for _, n := range graph.Nodes {
			dataJSON, _ := json.Marshal(n.Data)
			node := models.FlowNode{
				FlowID:    flowID,
				NodeID:    n.ID,
				Type:      n.Type,
				PositionX: n.Position["x"],
				PositionY: n.Position["y"],
				Data:      string(dataJSON),
			}
			if err := tx.Create(&node).Error; err != nil {
				return err
			}
		}

		// Insert new edges
		for _, e := range graph.Edges {
			edge := models.FlowEdge{
				FlowID:       flowID,
				EdgeID:       e.ID,
				Source:       e.Source,
				Target:       e.Target,
				SourceHandle: e.SourceHandle,
			}
			if err := tx.Create(&edge).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (h *WhatsAppHandler) getFlowGraph(flowID string) (string, error) {
	var nodes []models.FlowNode
	var edges []models.FlowEdge

	if err := database.GormDB.Where("flow_id = ?", flowID).Find(&nodes).Error; err != nil {
		return "", err
	}
	if err := database.GormDB.Where("flow_id = ?", flowID).Find(&edges).Error; err != nil {
		return "", err
	}

	graph := automation.FlowGraphData{
		Nodes: make([]automation.ReactFlowNode, len(nodes)),
		Edges: make([]automation.ReactFlowEdge, len(edges)),
	}

	for i, n := range nodes {
		var data automation.ReactFlowNodeData
		json.Unmarshal([]byte(n.Data), &data)
		graph.Nodes[i] = automation.ReactFlowNode{
			ID:   n.NodeID,
			Type: n.Type,
			Position: map[string]float64{
				"x": n.PositionX,
				"y": n.PositionY,
			},
			Data: data,
		}
	}

	for i, e := range edges {
		graph.Edges[i] = automation.ReactFlowEdge{
			ID:           e.EdgeID,
			Source:       e.Source,
			Target:       e.Target,
			SourceHandle: e.SourceHandle,
		}
	}

	graphJSON, _ := json.Marshal(graph)
	return string(graphJSON), nil
}

// GetLocalFlows lists local flows
func (h *WhatsAppHandler) GetLocalFlows(c *gin.Context) {
	var flows []models.Flow
	if err := database.GormDB.Order("updated_at DESC").Find(&flows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, flows)
}

// GetLocalFlow gets a single local flow
func (h *WhatsAppHandler) GetLocalFlow(c *gin.Context) {
	id := c.Param("id")
	var flow models.Flow
	if err := database.GormDB.First(&flow, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Flow not found"})
		return
	}

	graphJSON, err := h.getFlowGraph(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load graph data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         flow.ID,
		"name":       flow.Name,
		"graph_data": json.RawMessage(graphJSON),
	})
}

// DeleteLocalFlow deletes a local flow
func (h *WhatsAppHandler) DeleteLocalFlow(c *gin.Context) {
	id := c.Param("id")

	result := database.GormDB.Delete(&models.Flow{}, "id = ?", id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Flow not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
