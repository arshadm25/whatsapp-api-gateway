package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/pkg/models"

	"github.com/gin-gonic/gin"
)

type AutomationHandler struct{}

func NewAutomationHandler() *AutomationHandler {
	return &AutomationHandler{}
}

// GetRules returns all automation rules
func (h *AutomationHandler) GetRules(c *gin.Context) {
	rows, err := database.DB.Query(`
		SELECT id, name, type, enabled, priority, conditions, actions, created_at, updated_at
		FROM automation_rules
		ORDER BY priority DESC, created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var rules []models.AutomationRule
	for rows.Next() {
		var rule models.AutomationRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Type, &rule.Enabled, &rule.Priority,
			&rule.Conditions, &rule.Actions, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			log.Printf("Error scanning rule: %v", err)
			continue
		}
		rules = append(rules, rule)
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

	result, err := database.DB.Exec(`
		INSERT INTO automation_rules (name, type, priority, conditions, actions)
		VALUES (?, ?, ?, ?, ?)
	`, req.Name, req.Type, req.Priority, string(req.Conditions), string(req.Actions))

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusCreated, gin.H{"id": id, "message": "Rule created successfully"})
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

	_, err := database.DB.Exec(`
		UPDATE automation_rules 
		SET name = ?, type = ?, priority = ?, conditions = ?, actions = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, req.Name, req.Type, req.Priority, string(req.Conditions), string(req.Actions), id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rule updated successfully"})
}

// DeleteRule deletes an automation rule
func (h *AutomationHandler) DeleteRule(c *gin.Context) {
	id := c.Param("id")

	_, err := database.DB.Exec("DELETE FROM automation_rules WHERE id = ?", id)
	if err != nil {
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

	_, err := database.DB.Exec("UPDATE automation_rules SET enabled = ? WHERE id = ?", req.Enabled, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Rule toggled successfully"})
}

// GetLogs returns automation execution logs
func (h *AutomationHandler) GetLogs(c *gin.Context) {
	limit := c.DefaultQuery("limit", "50")
	limitInt, _ := strconv.Atoi(limit)

	rows, err := database.DB.Query(`
		SELECT id, rule_id, wa_id, trigger_type, action_taken, success, error_message, created_at
		FROM automation_logs
		ORDER BY created_at DESC
		LIMIT ?
	`, limitInt)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var logs []models.AutomationLog
	for rows.Next() {
		var log models.AutomationLog
		if err := rows.Scan(&log.ID, &log.RuleID, &log.WaID, &log.TriggerType,
			&log.ActionTaken, &log.Success, &log.ErrorMessage, &log.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	c.JSON(http.StatusOK, logs)
}

// GetAnalytics returns automation analytics
func (h *AutomationHandler) GetAnalytics(c *gin.Context) {
	var stats struct {
		TotalRules      int `json:"total_rules"`
		ActiveRules     int `json:"active_rules"`
		TotalExecutions int `json:"total_executions"`
		SuccessfulExecs int `json:"successful_executions"`
		FailedExecs     int `json:"failed_executions"`
	}

	database.DB.QueryRow("SELECT COUNT(*) FROM automation_rules").Scan(&stats.TotalRules)
	database.DB.QueryRow("SELECT COUNT(*) FROM automation_rules WHERE enabled = 1").Scan(&stats.ActiveRules)
	database.DB.QueryRow("SELECT COUNT(*) FROM automation_logs").Scan(&stats.TotalExecutions)
	database.DB.QueryRow("SELECT COUNT(*) FROM automation_logs WHERE success = 1").Scan(&stats.SuccessfulExecs)
	database.DB.QueryRow("SELECT COUNT(*) FROM automation_logs WHERE success = 0").Scan(&stats.FailedExecs)

	c.JSON(http.StatusOK, stats)
}
