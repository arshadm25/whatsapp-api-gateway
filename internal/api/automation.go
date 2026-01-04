package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"

	"github.com/gin-gonic/gin"
)

type AutomationHandler struct{}

func NewAutomationHandler() *AutomationHandler {
	return &AutomationHandler{}
}

// GetRules returns all automation rules
func (h *AutomationHandler) GetRules(c *gin.Context) {
	var rules []models.AutomationRule
	if err := database.GormDB.Order("priority DESC, created_at DESC").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rules)
}

// CreateRule creates a new automation rule
func (h *AutomationHandler) CreateRule(c *gin.Context) {
	var req struct {
		Name       string          `json:"name" binding:"required"`
		Type       string          `json:"type" binding:"required"`
		Priority   int             `json:"priority"`
		Conditions json.RawMessage `json:"conditions" binding:"required"`
		Actions    json.RawMessage `json:"actions" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := models.AutomationRule{
		Name:       req.Name,
		Type:       req.Type,
		Priority:   req.Priority,
		Conditions: string(req.Conditions),
		Actions:    string(req.Actions),
	}

	if err := database.GormDB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": rule.ID, "message": "Rule created successfully"})
}

// UpdateRule updates an existing automation rule
func (h *AutomationHandler) UpdateRule(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name       string          `json:"name"`
		Type       string          `json:"type"`
		Priority   int             `json:"priority"`
		Conditions json.RawMessage `json:"conditions"`
		Actions    json.RawMessage `json:"actions"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateData := map[string]interface{}{}
	if req.Name != "" {
		updateData["name"] = req.Name
	}
	if req.Type != "" {
		updateData["type"] = req.Type
	}
	updateData["priority"] = req.Priority
	if len(req.Conditions) > 0 {
		updateData["conditions"] = string(req.Conditions)
	}
	if len(req.Actions) > 0 {
		updateData["actions"] = string(req.Actions)
	}

	if err := database.GormDB.Model(&models.AutomationRule{}).Where("id = ?", id).Updates(updateData).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rule updated successfully"})
}

// DeleteRule deletes an automation rule
func (h *AutomationHandler) DeleteRule(c *gin.Context) {
	id := c.Param("id")

	if err := database.GormDB.Delete(&models.AutomationRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rule deleted successfully"})
}

// ToggleRule enables or disables a rule
func (h *AutomationHandler) ToggleRule(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.GormDB.Model(&models.AutomationRule{}).Where("id = ?", id).Update("enabled", req.Enabled).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rule toggled successfully"})
}

// GetLogs returns automation execution logs
func (h *AutomationHandler) GetLogs(c *gin.Context) {
	limit := c.DefaultQuery("limit", "50")
	limitInt, _ := strconv.Atoi(limit)

	var logs []models.AutomationLog
	if err := database.GormDB.Order("created_at DESC").Limit(limitInt).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, logs)
}

// GetAnalytics returns automation analytics
func (h *AutomationHandler) GetAnalytics(c *gin.Context) {
	var stats struct {
		TotalRules      int64 `json:"total_rules"`
		ActiveRules     int64 `json:"active_rules"`
		TotalExecutions int64 `json:"total_executions"`
		SuccessfulExecs int64 `json:"successful_executions"`
		FailedExecs     int64 `json:"failed_executions"`
	}

	database.GormDB.Model(&models.AutomationRule{}).Count(&stats.TotalRules)
	database.GormDB.Model(&models.AutomationRule{}).Where("enabled = ?", true).Count(&stats.ActiveRules)
	database.GormDB.Model(&models.AutomationLog{}).Count(&stats.TotalExecutions)
	database.GormDB.Model(&models.AutomationLog{}).Where("success = ?", true).Count(&stats.SuccessfulExecs)
	database.GormDB.Model(&models.AutomationLog{}).Where("success = ?", false).Count(&stats.FailedExecs)

	c.JSON(http.StatusOK, stats)
}

// GetActiveSessions returns all currently active chatbot sessions
func (h *AutomationHandler) GetActiveSessions(c *gin.Context) {
	type SessionInfo struct {
		ID          uint      `json:"id"`
		WaID        string    `json:"wa_id"`
		ContactName string    `json:"contact_name"`
		FlowID      string    `json:"flow_id"`
		FlowName    string    `json:"flow_name"`
		CurrentNode string    `json:"current_node"`
		Status      string    `json:"status"`
		StartedAt   time.Time `json:"started_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	var sessions []SessionInfo
	err := database.GormDB.Table("conversation_sessions").
		Select("conversation_sessions.*, contacts.name as contact_name, flows.name as flow_name").
		Joins("LEFT JOIN contacts ON conversation_sessions.wa_id = contacts.wa_id").
		Joins("LEFT JOIN flows ON conversation_sessions.flow_id = flows.id").
		Where("conversation_sessions.status = ?", "active").
		Order("conversation_sessions.updated_at DESC").
		Scan(&sessions).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, sessions)
}

// GetSessionMessages returns messages for a specific session
func (h *AutomationHandler) GetSessionMessages(c *gin.Context) {
	waID := c.Param("wa_id")
	startedAtStr := c.Query("started_at")

	// Filter messages for this contact (both sent and received)
	query := database.GormDB.Where("wa_id = ? OR sender = ?", waID, waID)

	if startedAtStr != "" {
		startedAt, err := time.Parse(time.RFC3339, startedAtStr)
		if err == nil {
			// Only show messages from this session
			query = query.Where("created_at >= ?", startedAt.Add(-1*time.Second))
		}
	}

	var messages []models.Message
	if err := query.Order("created_at ASC").Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, messages)
}

// TerminateSession forcefully ends an active session
func (h *AutomationHandler) TerminateSession(c *gin.Context) {
	id := c.Param("id")

	if err := database.GormDB.Model(&models.ConversationSession{}).
		Where("id = ?", id).
		Update("status", "terminated").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session terminated successfully"})
}

// GetSettings returns all system settings
func (h *AutomationHandler) GetSettings(c *gin.Context) {
	var settings []models.SystemSetting
	if err := database.GormDB.Find(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSetting updates a specific system setting
func (h *AutomationHandler) UpdateSetting(c *gin.Context) {
	var req struct {
		Key   string `json:"key" binding:"required"`
		Value string `json:"value" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.GormDB.Model(&models.SystemSetting{}).
		Where("key = ?", req.Key).
		Update("value", req.Value).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Setting updated successfully. Please restart server for some changes to take effect."})
}
