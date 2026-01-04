package automation

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"
	"whatsapp-gateway/internal/whatsapp"
	"whatsapp-gateway/internal/ws"

	"gorm.io/gorm"
)

type Engine struct {
	WhatsAppClient *whatsapp.Client
	Hub            *ws.Hub
}

func NewEngine(client *whatsapp.Client, hub *ws.Hub) *Engine {
	return &Engine{WhatsAppClient: client, Hub: hub}
}

// Condition represents a rule condition
type Condition struct {
	Type     string `json:"type"`     // keyword, time, contact_tag, message_type
	Operator string `json:"operator"` // equals, contains, regex, between
	Value    string `json:"value"`
}

// Action represents an automation action
type Action struct {
	Type   string                 `json:"type"`   // send_message, add_tag, start_flow
	Params map[string]interface{} `json:"params"` // action-specific parameters
}

// ProcessIncomingMessage processes a message through automation rules
func (e *Engine) ProcessIncomingMessage(waID, messageContent string) error {
	// 0. Check if user is in an active Flow Session
	var session models.ConversationSession
	err := database.GormDB.Where("wa_id = ? AND status = 'active'", waID).First(&session).Error

	if err == nil {
		// Active, continue flow
		log.Printf("[Flow] Continuing flow %s for %s at node %s", session.FlowID, waID, session.CurrentNode)
		return e.ContinueFlow(waID, int(session.ID), session.FlowID, session.CurrentNode, messageContent)
	}

	// 1. Fetch all enabled rules ordered by priority
	var rules []models.AutomationRule
	if err := database.GormDB.Where("enabled = ?", true).Order("priority DESC, created_at DESC").Find(&rules).Error; err != nil {
		log.Printf("Error fetching automation rules: %v", err)
		return err
	}

	for _, rule := range rules {
		// Check if rule conditions match
		if e.evaluateConditions(rule.Conditions, waID, messageContent) {
			log.Printf("Rule '%s' matched for message from %s", rule.Name, waID)

			// Execute actions
			if err := e.executeActions(int(rule.ID), rule.Actions, waID, messageContent); err != nil {
				log.Printf("Error executing actions for rule %s: %v", rule.Name, err)
				e.logAutomation(int(rule.ID), waID, rule.Type, "action_failed", false, err.Error())
			} else {
				e.logAutomation(int(rule.ID), waID, rule.Type, "action_executed", true, "")
			}

			// For now, stop after first matching rule (can be configurable)
			break
		}
	}

	// TEMPORARY: Hardcoded Trigger for testing new Flows
	// If message is "test" or "start", start the latest edited flow
	if strings.ToLower(messageContent) == "test" || strings.ToLower(messageContent) == "start" {
		var latestFlow models.Flow
		err := database.GormDB.Order("updated_at DESC").First(&latestFlow).Error
		if err == nil && latestFlow.ID != "" {
			log.Printf("[TEST] Starting latest flow: %s", latestFlow.ID)
			return e.StartFlow(waID, latestFlow.ID)
		}
	}

	return nil
}

// evaluateConditions checks if all conditions are met
func (e *Engine) evaluateConditions(conditionsJSON, waID, messageContent string) bool {
	var conditions []Condition
	if err := json.Unmarshal([]byte(conditionsJSON), &conditions); err != nil {
		log.Printf("Error parsing conditions: %v", err)
		return false
	}

	// All conditions must be true (AND logic)
	for _, cond := range conditions {
		if !e.evaluateSingleCondition(cond, waID, messageContent) {
			return false
		}
	}

	return true
}

// evaluateSingleCondition evaluates a single condition
func (e *Engine) evaluateSingleCondition(cond Condition, waID, messageContent string) bool {
	switch cond.Type {
	case "keyword":
		return e.matchKeyword(messageContent, cond.Operator, cond.Value)
	case "message_type":
		// For now, we only handle text messages
		return cond.Value == "text"
	case "contact_tag":
		return e.hasContactTag(waID, cond.Value)
	default:
		log.Printf("Unknown condition type: %s", cond.Type)
		return false
	}
}

// matchKeyword checks if message matches keyword condition
func (e *Engine) matchKeyword(message, operator, value string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	value = strings.ToLower(value)

	switch operator {
	case "equals":
		return message == value
	case "contains":
		return strings.Contains(message, value)
	case "starts_with":
		return strings.HasPrefix(message, value)
	case "regex":
		matched, err := regexp.MatchString(value, message)
		if err != nil {
			log.Printf("Regex error: %v", err)
			return false
		}
		return matched
	default:
		return false
	}
}

// hasContactTag checks if contact has a specific tag
func (e *Engine) hasContactTag(waID, tag string) bool {
	var contact models.Contact
	err := database.GormDB.Select("tags").Where("wa_id = ?", waID).First(&contact).Error
	if err != nil {
		return false
	}
	return strings.Contains(contact.Tags, tag)
}

// executeActions executes all actions for a matched rule
func (e *Engine) executeActions(ruleID int, actionsJSON, waID, messageContent string) error {
	var actions []Action
	if err := json.Unmarshal([]byte(actionsJSON), &actions); err != nil {
		return err
	}

	for _, action := range actions {
		if err := e.executeSingleAction(action, waID, messageContent); err != nil {
			return err
		}
	}

	return nil
}

// executeSingleAction executes a single action
func (e *Engine) executeSingleAction(action Action, waID, messageContent string) error {
	switch action.Type {
	case "send_message":
		message, ok := action.Params["message"].(string)
		if !ok {
			return nil
		}
		// Replace variables in message
		message = strings.ReplaceAll(message, "{{contact_name}}", waID)
		message = strings.ReplaceAll(message, "{{message}}", messageContent)

		return e.WhatsAppClient.SendMessage(waID, message)

	case "add_tag":
		tag, ok := action.Params["tag"].(string)
		if !ok {
			return nil
		}
		return e.addTagToContact(waID, tag)

	case "start_flow":
		// Support new string-based Flow IDs (UUIDs)
		if flowID, ok := action.Params["flow_id"].(string); ok {
			return e.StartFlow(waID, flowID)
		}

		// Legacy support (integer IDs)
		if flowID, ok := action.Params["flow_id"].(float64); ok {
			return e.startChatbotFlow(waID, int(flowID))
		}
		return nil

	default:
		log.Printf("Unknown action type: %s", action.Type)
	}

	return nil
}

// addTagToContact adds a tag to a contact
func (e *Engine) addTagToContact(waID, tag string) error {
	var contact models.Contact
	err := database.GormDB.Where("wa_id = ?", waID).First(&contact).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		contact = models.Contact{WaID: waID, Tags: "[]"}
	}

	var tags []string
	json.Unmarshal([]byte(contact.Tags), &tags)

	// Check if tag already exists
	for _, t := range tags {
		if t == tag {
			return nil // Tag already exists
		}
	}

	tags = append(tags, tag)
	newTags, _ := json.Marshal(tags)
	contact.Tags = string(newTags)

	return database.GormDB.Save(&contact).Error
}

// startChatbotFlow initiates a chatbot conversation flow
func (e *Engine) startChatbotFlow(waID string, flowID int) error {
	session := models.ConversationSession{
		WaID:        waID,
		FlowID:      fmt.Sprintf("%d", flowID),
		CurrentNode: "start",
		Context:     "{}",
		Status:      "active",
	}

	err := database.GormDB.Create(&session).Error
	if err != nil {
		return err
	}

	log.Printf("Started flow %d for contact %s", flowID, waID)
	return nil
}

// logAutomation logs automation execution
func (e *Engine) logAutomation(ruleID int, waID, triggerType, actionTaken string, success bool, errorMsg string) {
	logEntry := models.AutomationLog{
		RuleID:       uint(ruleID),
		WaID:         waID,
		TriggerType:  triggerType,
		ActionTaken:  actionTaken,
		Success:      success,
		ErrorMessage: errorMsg,
	}
	database.GormDB.Create(&logEntry)
}
