package automation

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/whatsapp"
	"whatsapp-gateway/pkg/models"
)

type Engine struct {
	WhatsAppClient *whatsapp.Client
}

func NewEngine(client *whatsapp.Client) *Engine {
	return &Engine{WhatsAppClient: client}
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
	// Fetch all enabled rules ordered by priority
	rows, err := database.DB.Query(`
		SELECT id, name, type, conditions, actions 
		FROM automation_rules 
		WHERE enabled = 1 
		ORDER BY priority DESC
	`)
	if err != nil {
		log.Printf("Error fetching automation rules: %v", err)
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var rule models.AutomationRule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Type, &rule.Conditions, &rule.Actions); err != nil {
			log.Printf("Error scanning rule: %v", err)
			continue
		}

		// Check if rule conditions match
		if e.evaluateConditions(rule.Conditions, waID, messageContent) {
			log.Printf("Rule '%s' matched for message from %s", rule.Name, waID)

			// Execute actions
			if err := e.executeActions(rule.ID, rule.Actions, waID, messageContent); err != nil {
				log.Printf("Error executing actions for rule %s: %v", rule.Name, err)
				e.logAutomation(rule.ID, waID, rule.Type, "action_failed", false, err.Error())
			} else {
				e.logAutomation(rule.ID, waID, rule.Type, "action_executed", true, "")
			}

			// For now, stop after first matching rule (can be configurable)
			break
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
	var tags string
	err := database.DB.QueryRow("SELECT tags FROM contacts WHERE wa_id = ?", waID).Scan(&tags)
	if err != nil {
		return false
	}
	return strings.Contains(tags, tag)
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
		flowID, ok := action.Params["flow_id"].(float64)
		if !ok {
			return nil
		}
		return e.startChatbotFlow(waID, int(flowID))

	default:
		log.Printf("Unknown action type: %s", action.Type)
	}

	return nil
}

// addTagToContact adds a tag to a contact
func (e *Engine) addTagToContact(waID, tag string) error {
	var currentTags string
	err := database.DB.QueryRow("SELECT tags FROM contacts WHERE wa_id = ?", waID).Scan(&currentTags)
	if err != nil {
		currentTags = "[]"
	}

	var tags []string
	json.Unmarshal([]byte(currentTags), &tags)

	// Check if tag already exists
	for _, t := range tags {
		if t == tag {
			return nil // Tag already exists
		}
	}

	tags = append(tags, tag)
	newTags, _ := json.Marshal(tags)

	_, err = database.DB.Exec("UPDATE contacts SET tags = ? WHERE wa_id = ?", string(newTags), waID)
	return err
}

// startChatbotFlow initiates a chatbot conversation flow
func (e *Engine) startChatbotFlow(waID string, flowID int) error {
	// Create a new conversation session
	_, err := database.DB.Exec(`
		INSERT INTO conversation_sessions (wa_id, flow_id, current_node, context, status)
		VALUES (?, ?, 'start', '{}', 'active')
	`, waID, flowID)

	if err != nil {
		return err
	}

	// TODO: Send first message from flow
	log.Printf("Started flow %d for contact %s", flowID, waID)
	return nil
}

// logAutomation logs automation execution
func (e *Engine) logAutomation(ruleID int, waID, triggerType, actionTaken string, success bool, errorMsg string) {
	database.DB.Exec(`
		INSERT INTO automation_logs (rule_id, wa_id, trigger_type, action_taken, success, error_message)
		VALUES (?, ?, ?, ?, ?, ?)
	`, ruleID, waID, triggerType, actionTaken, success, errorMsg)
}
