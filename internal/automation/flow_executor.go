package automation

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"whatsapp-gateway/internal/database"
)

// StartFlow initiates a flow for a user
func (e *Engine) StartFlow(waID string, flowID string) error {
	// 1. Fetch Flow Data
	var graphDataJSON string
	err := database.DB.QueryRow("SELECT graph_data FROM flows WHERE id = ?", flowID).Scan(&graphDataJSON)
	if err != nil {
		log.Printf("Error fetching flow %s: %v", flowID, err)
		return err
	}

	// 2. Parse Graph Data
	var graph FlowGraphData
	if err := json.Unmarshal([]byte(graphDataJSON), &graph); err != nil {
		log.Printf("Error parsing flow graph: %v", err)
		return err
	}

	// 3. Find Start Node
	var startNode *ReactFlowNode
	for _, node := range graph.Nodes {
		if node.Data.IsStart {
			startNode = &node
			break
		}
	}

	if startNode == nil {
		return fmt.Errorf("no start node found in flow %s", flowID)
	}

	// 4. Create Session
	_, err = database.DB.Exec(`
		INSERT INTO conversation_sessions (wa_id, flow_id, current_node, context, status)
		VALUES (?, ?, ?, '{}', 'active')
	`, waID, flowID, startNode.ID)

	if err != nil {
		// If existing active session, update it.
		e.TerminateSession(waID)
		// Retry
		_, err = database.DB.Exec(`
			INSERT INTO conversation_sessions (wa_id, flow_id, current_node, context, status)
			VALUES (?, ?, ?, '{}', 'active')
		`, waID, flowID, startNode.ID)
		if err != nil {
			return err
		}
	}

	// 5. Execute Start Node
	return e.ExecuteNode(waID, *startNode, graph)
}

// ContinueFlow handles user input in an active flow
func (e *Engine) ContinueFlow(waID string, sessionID int, flowID, currentNodeID string, messageContent string) error {
	// 1. Fetch Flow Data
	var graphDataJSON string
	err := database.DB.QueryRow("SELECT graph_data FROM flows WHERE id = ?", flowID).Scan(&graphDataJSON)
	if err != nil {
		return err
	}

	var graph FlowGraphData
	if err := json.Unmarshal([]byte(graphDataJSON), &graph); err != nil {
		return err
	}

	// 2. Find Current Node
	var currentNode *ReactFlowNode
	for _, node := range graph.Nodes {
		if node.ID == currentNodeID {
			currentNode = &node
			break
		}
	}
	if currentNode == nil {
		e.TerminateSessionByID(sessionID)
		return fmt.Errorf("node %s not found", currentNodeID)
	}

	// 3. Handle Input (Validation & Storage)

	// Determine validation rules from the last step (which usually is the input trigger)
	var validation *StepValidation
	var variableName string
	var stepType string

	if len(currentNode.Data.Steps) > 0 {
		lastStep := currentNode.Data.Steps[len(currentNode.Data.Steps)-1]
		validation = lastStep.Validation
		variableName = lastStep.Variable
		stepType = lastStep.Type
	}

	// Check Validation
	isValid := true
	errorMessage := "Invalid input. Please try again."

	if validation != nil {
		// 3.1 Validate based on rules
		// Max Retries Logic
		maxRetries := 3 // default
		if validMax, ok := ToInt(validation.MaxRetries); ok {
			maxRetries = validMax
		}

		if validation.ErrorMessage != "" {
			errorMessage = validation.ErrorMessage
		}

		// Regex / Content Validation
		isValid = e.ValidateInput(messageContent, stepType, validation)

		if !isValid {
			// Handle Retry Count
			retryKey := fmt.Sprintf("%s_retries", currentNodeID)
			currentRetries := e.GetContextInt(sessionID, retryKey)

			if currentRetries < maxRetries {
				// Send Error Message
				e.WhatsAppClient.SendMessage(waID, errorMessage)
				// Increment Retries
				e.UpdateSessionContext(sessionID, retryKey, fmt.Sprintf("%d", currentRetries+1))
				// Stay on current node (Return)
				return nil
			} else {
				// Retries exhausted
				// Option A: Stop Flow
				e.WhatsAppClient.SendMessage(waID, "Too many invalid attempts. Session ended.")
				e.TerminateSessionByID(sessionID)
				return nil
				// Option B: Fallback (Future improvement)
			}
		} else {
			// Reset retry count on success
			e.UpdateSessionContext(sessionID, fmt.Sprintf("%s_retries", currentNodeID), "0")
		}
	}

	if isValid {
		// Save variable if present
		if variableName != "" {
			e.UpdateSessionContext(sessionID, variableName, messageContent)
		}

		// 4. Find Next Node via Edges
		nextNodeID := e.FindNextNodeID(currentNode, graph.Edges, messageContent)

		if nextNodeID != "" {
			// Update Session
			database.DB.Exec("UPDATE conversation_sessions SET current_node = ? WHERE id = ?", nextNodeID, sessionID)

			// Execute Next Node
			var nextNode ReactFlowNode
			for _, n := range graph.Nodes {
				if n.ID == nextNodeID {
					nextNode = n
					break
				}
			}
			return e.ExecuteNode(waID, nextNode, graph)
		} else {
			// End of Flow?
			e.TerminateSessionByID(sessionID)
			return nil
		}
	}

	return nil
}

func (e *Engine) ValidateInput(input, stepType string, validation *StepValidation) bool {
	// Standard Regex Check
	if validation.Regex != "" {
		match, err := regexp.MatchString(validation.Regex, input)
		if err == nil && !match {
			return false
		}
	}

	// Number Input: Min/Max
	if stepType == "Number Input" {
		val, err := strconv.ParseFloat(input, 64)
		if err != nil {
			return false // Not a number
		}

		if validation.Min != nil {
			if minVal, ok := ToFloat(validation.Min); ok {
				if val < minVal {
					return false
				}
			}
		}
		if validation.Max != nil {
			if maxVal, ok := ToFloat(validation.Max); ok {
				if val > maxVal {
					return false
				}
			}
		}
	}

	// Email Input Basic Check (if strict regex not provided)
	if stepType == "Email Input" && validation.Regex == "" {
		if !strings.Contains(input, "@") || !strings.Contains(input, ".") {
			return false
		}
	}

	return true
}

func (e *Engine) FindNextNodeID(currentNode *ReactFlowNode, edges []ReactFlowEdge, input string) string {
	// 1. Check if node has Quick Replies
	hasQuickReplies := false
	for _, step := range currentNode.Data.Steps {
		if step.Type == "Quick Reply" {
			hasQuickReplies = true
			break
		}
	}

	if hasQuickReplies {
		// Match input to button label
		for sIdx, step := range currentNode.Data.Steps {
			if step.Type == "Quick Reply" {
				for bIdx, btn := range step.Buttons {
					if strings.EqualFold(btn.Label, input) {
						// Found button! Look for edge from sourceHandle `handle-{sIdx}-{bIdx}`
						handleID := fmt.Sprintf("handle-%d-%d", sIdx, bIdx)
						for _, edge := range edges {
							if edge.Source == currentNode.ID && edge.SourceHandle == handleID {
								return edge.Target
							}
						}
					}
				}
			}
		}
	}

	// 2. Default Navigation
	for _, edge := range edges {
		if edge.Source == currentNode.ID {
			// Generic edge
			if !hasQuickReplies || edge.SourceHandle == "" || strings.HasSuffix(edge.SourceHandle, "default") {
				return edge.Target
			}
		}
	}

	return ""
}

func (e *Engine) ExecuteNode(waID string, node ReactFlowNode, graph FlowGraphData) error {
	// Iterate through steps and execute them
	for _, step := range node.Data.Steps {
		switch step.Type {
		case "Text", "Text Message":
			text := e.ReplaceVariables(waID, step.Content)
			e.WhatsAppClient.SendMessage(waID, text)

		case "Quick Reply":
			// Send Text with options listed (Fallback for now)
			text := step.Content + "\n\nOptions:"
			for _, btn := range step.Buttons {
				text += "\n- " + btn.Label
			}
			e.WhatsAppClient.SendMessage(waID, text)

		case "Image":
			e.WhatsAppClient.SendMessage(waID, "[Image] "+step.Content)

		case "Text Input", "Number Input", "Email Input":
			// Usually we don't do anything here, just wait.
			// But if the step has a prompt (step.Content), we could send it?
			// The Builder doesn't seem to enforce Content on Input steps, it uses previous Text node.
			// However, if Step.Content exists, send it.
			if step.Content != "" {
				text := e.ReplaceVariables(waID, step.Content)
				e.WhatsAppClient.SendMessage(waID, text)
			}
		}
	}

	// If this node ends with an Input Step, we STOP here.
	if len(node.Data.Steps) > 0 {
		lastStep := node.Data.Steps[len(node.Data.Steps)-1]
		if strings.Contains(lastStep.Type, "Input") {
			return nil // Stop and wait.
		}
		if lastStep.Type == "Quick Reply" {
			return nil // Stop and wait.
		}
	}

	// If NOT waiting for input, automatically move to next Node
	nextNodeID := e.FindNextNodeID(&node, graph.Edges, "")
	if nextNodeID != "" {
		var sessionID int
		database.DB.QueryRow("SELECT id FROM conversation_sessions WHERE wa_id = ? AND status='active'", waID).Scan(&sessionID)
		database.DB.Exec("UPDATE conversation_sessions SET current_node = ? WHERE id = ?", nextNodeID, sessionID)

		var nextNode ReactFlowNode
		for _, n := range graph.Nodes {
			if n.ID == nextNodeID {
				nextNode = n
				break
			}
		}
		return e.ExecuteNode(waID, nextNode, graph)
	} else {
		// End of Flow
		var sessionID int
		database.DB.QueryRow("SELECT id FROM conversation_sessions WHERE wa_id = ? AND status='active'", waID).Scan(&sessionID)
		e.TerminateSessionByID(sessionID)
	}

	return nil
}

func (e *Engine) TerminateSession(waID string) {
	database.DB.Exec("UPDATE conversation_sessions SET status = 'completed' WHERE wa_id = ? AND status = 'active'", waID)
}

func (e *Engine) TerminateSessionByID(id int) {
	database.DB.Exec("UPDATE conversation_sessions SET status = 'completed' WHERE id = ?", id)
}

func (e *Engine) UpdateSessionContext(sessionID int, key, value string) {
	var contextJSON string
	database.DB.QueryRow("SELECT context FROM conversation_sessions WHERE id = ?", sessionID).Scan(&contextJSON)

	var context map[string]string
	if contextJSON == "" {
		context = make(map[string]string)
	} else {
		json.Unmarshal([]byte(contextJSON), &context)
	}

	context[key] = value

	newContextJSON, _ := json.Marshal(context)
	database.DB.Exec("UPDATE conversation_sessions SET context = ? WHERE id = ?", string(newContextJSON), sessionID)
}

func (e *Engine) GetContextInt(sessionID int, key string) int {
	var contextJSON string
	database.DB.QueryRow("SELECT context FROM conversation_sessions WHERE id = ?", sessionID).Scan(&contextJSON)

	if contextJSON == "" {
		return 0
	}

	var context map[string]string
	if err := json.Unmarshal([]byte(contextJSON), &context); err != nil {
		return 0
	}

	valStr, exists := context[key]
	if !exists {
		return 0
	}

	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0
	}
	return val
}

func (e *Engine) ReplaceVariables(waID string, text string) string {
	// 1. Get Contact Info
	var name string
	database.DB.QueryRow("SELECT name FROM contacts WHERE wa_id = ?", waID).Scan(&name)
	text = strings.ReplaceAll(text, "{{contact.name}}", name)
	text = strings.ReplaceAll(text, "{{contact.phone}}", waID)

	// 2. Get Session Context
	var contextJSON string
	database.DB.QueryRow("SELECT context FROM conversation_sessions WHERE wa_id = ? AND status='active'", waID).Scan(&contextJSON)

	if contextJSON != "" {
		var context map[string]string
		json.Unmarshal([]byte(contextJSON), &context)
		for k, v := range context {
			text = strings.ReplaceAll(text, "{{vars."+k+"}}", v)
		}
	}
	return text
}

// Helpers for Interface Conversion

func ToInt(v interface{}) (int, bool) {
	if val, ok := v.(float64); ok { // JSON numbers are float64
		return int(val), true
	}
	if val, ok := v.(string); ok {
		if res, err := strconv.Atoi(val); err == nil {
			return res, true
		}
	}
	return 0, false
}

func ToFloat(v interface{}) (float64, bool) {
	if val, ok := v.(float64); ok {
		return val, true
	}
	if val, ok := v.(string); ok {
		if res, err := strconv.ParseFloat(val, 64); err == nil {
			return res, true
		}
	}
	return 0, false
}
