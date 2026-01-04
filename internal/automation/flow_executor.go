package automation

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"
	"whatsapp-gateway/internal/whatsapp"
)

// StartFlow initiates a flow for a user
func (e *Engine) StartFlow(waID string, flowID string) error {
	// 1. Fetch Graph Data Relationally
	graph, err := e.LoadGraph(flowID)
	if err != nil {
		log.Printf("Error loading graph for flow %s: %v", flowID, err)
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
	session := models.ConversationSession{
		WaID:        waID,
		FlowID:      flowID,
		CurrentNode: startNode.ID,
		Context:     "{}",
		Status:      "active",
	}

	if err := database.GormDB.Create(&session).Error; err != nil {
		// If existing active session, terminate it and try again.
		e.TerminateSession(waID)
		if err := database.GormDB.Create(&session).Error; err != nil {
			return err
		}
	}

	// 5. Execute Start Node
	return e.ExecuteNode(waID, *startNode, *graph)
}

// ContinueFlow handles user input in an active flow
func (e *Engine) ContinueFlow(waID string, sessionID int, flowID, currentNodeID string, messageContent string) error {
	log.Printf("[ContinueFlow] waID=%s, sessionID=%d, flowID=%s, currentNodeID=%s, messageContent='%s'",
		waID, sessionID, flowID, currentNodeID, messageContent)

	// 1. Fetch Graph Data Relationally
	graph, err := e.LoadGraph(flowID)
	if err != nil {
		log.Printf("Error loading graph for flow %s: %v", flowID, err)
		return err
	}

	// Log all nodes in graph for debugging
	nodeIDs := []string{}
	for _, node := range graph.Nodes {
		nodeIDs = append(nodeIDs, node.ID)
	}
	log.Printf("[ContinueFlow] Graph has %d nodes: %v", len(graph.Nodes), nodeIDs)

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

	// Always validate Email and Number inputs (even without explicit validation config)
	if stepType == "Email Input" {
		if !strings.Contains(messageContent, "@") || !strings.Contains(messageContent, ".") {
			isValid = false
			errorMessage = "Please enter a valid email address."
		}
	}

	if stepType == "Number Input" {
		if _, err := strconv.ParseFloat(messageContent, 64); err != nil {
			isValid = false
			errorMessage = "Please enter a valid number."
		}
	}

	// Apply custom validation rules if configured
	maxRetries := 3 // default
	if validation != nil {
		// Get custom max retries if configured
		if validMax, ok := ToInt(validation.MaxRetries); ok {
			maxRetries = validMax
		}

		// Get custom error message if configured
		if validation.ErrorMessage != "" {
			errorMessage = validation.ErrorMessage
		}

		// Regex / Content Validation (this may override the basic validation above)
		validationResult := e.ValidateInput(messageContent, stepType, validation)
		if !validationResult {
			isValid = false
		}
	}

	// Handle validation result (retry logic applies to all inputs)
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
			e.WhatsAppClient.SendMessage(waID, "Too many invalid attempts. Session ended.")
			e.TerminateSessionByID(sessionID)
			return nil
		}
	} else {
		// Reset retry count on success
		e.UpdateSessionContext(sessionID, fmt.Sprintf("%s_retries", currentNodeID), "0")
	}

	if isValid {
		// Save variable if present
		if variableName != "" {
			e.UpdateSessionContext(sessionID, variableName, messageContent)
		}

		// 4. Find Next Node via Edges
		log.Printf("[ContinueFlow] Finding next node for current node: %s", currentNodeID)
		nextNodeID := e.FindNextNodeID(currentNode, graph.Edges, messageContent)
		log.Printf("[ContinueFlow] Next node ID: %s", nextNodeID)

		if nextNodeID != "" {
			// Update Session
			database.GormDB.Model(&models.ConversationSession{}).Where("id = ?", sessionID).Update("current_node", nextNodeID)

			// Execute Next Node
			var nextNode *ReactFlowNode
			for _, n := range graph.Nodes {
				if n.ID == nextNodeID {
					nextNode = &n
					break
				}
			}

			if nextNode == nil {
				log.Printf("[ContinueFlow] ERROR: Next node %s not found in graph!", nextNodeID)
				e.TerminateSessionByID(sessionID)
				return fmt.Errorf("next node not found: %s", nextNodeID)
			}

			log.Printf("[ContinueFlow] Executing next node: %s (label: %s)", nextNodeID, nextNode.Data.Label)
			return e.ExecuteNode(waID, *nextNode, *graph)
		} else {
			// End of Flow?
			log.Printf("[ContinueFlow] No next node found, terminating session")
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
	log.Printf("[FindNextNodeID] Current Node: %s, Input: '%s'", currentNode.ID, input)

	// 1. Check if node has Quick Replies or Lists
	hasQuickReplies := false
	hasList := false
	for _, step := range currentNode.Data.Steps {
		if step.Type == "Quick Reply" {
			hasQuickReplies = true
			break
		}
		if step.Type == "List" {
			hasList = true
			break
		}
	}

	if hasQuickReplies {
		log.Printf("[FindNextNodeID] Node has Quick Replies, matching input...")
		// Match input to button label
		for sIdx, step := range currentNode.Data.Steps {
			if step.Type == "Quick Reply" {
				for bIdx, btn := range step.Buttons {
					log.Printf("[FindNextNodeID] Checking button[%d][%d]: '%s' vs input: '%s'", sIdx, bIdx, btn.Label, input)
					if strings.EqualFold(btn.Label, input) {
						// Found button! Look for edge from sourceHandle `handle-{sIdx}-{bIdx}`
						handleID := fmt.Sprintf("handle-%d-%d", sIdx, bIdx)
						log.Printf("[FindNextNodeID] Button matched! Looking for edge with sourceHandle: %s", handleID)

						for _, edge := range edges {
							log.Printf("[FindNextNodeID] Checking edge: source=%s, target=%s, sourceHandle=%s", edge.Source, edge.Target, edge.SourceHandle)
							if edge.Source == currentNode.ID && edge.SourceHandle == handleID {
								log.Printf("[FindNextNodeID] Found matching edge! Target: %s", edge.Target)
								return edge.Target
							}
						}
						log.Printf("[FindNextNodeID] No edge found for handle: %s", handleID)
					}
				}
			}
		}
	}

	if hasList {
		log.Printf("[FindNextNodeID] Node has List, matching input...")
		// Match input to list option title
		for sIdx, step := range currentNode.Data.Steps {
			if step.Type == "List" {
				for oIdx, opt := range step.Options {
					log.Printf("[FindNextNodeID] Checking option[%d][%d]: '%s' vs input: '%s'", sIdx, oIdx, opt.Title, input)
					if strings.EqualFold(opt.Title, input) {
						// Found option! Look for edge from sourceHandle `handle-{sIdx}-{oIdx}`
						handleID := fmt.Sprintf("handle-%d-%d", sIdx, oIdx)
						log.Printf("[FindNextNodeID] Option matched! Looking for edge with sourceHandle: %s", handleID)

						for _, edge := range edges {
							log.Printf("[FindNextNodeID] Checking edge: source=%s, target=%s, sourceHandle=%s", edge.Source, edge.Target, edge.SourceHandle)
							if edge.Source == currentNode.ID && edge.SourceHandle == handleID {
								log.Printf("[FindNextNodeID] Found matching edge! Target: %s", edge.Target)
								return edge.Target
							}
						}
						log.Printf("[FindNextNodeID] No edge found for handle: %s", handleID)
					}
				}
			}
		}
	}

	// 2. Default Navigation
	log.Printf("[FindNextNodeID] Checking default navigation...")
	for _, edge := range edges {
		if edge.Source == currentNode.ID {
			log.Printf("[FindNextNodeID] Found edge from current node: sourceHandle='%s'", edge.SourceHandle)
			// Generic edge
			if (!hasQuickReplies && !hasList) || edge.SourceHandle == "" || strings.HasSuffix(edge.SourceHandle, "default") {
				log.Printf("[FindNextNodeID] Using default edge, target: %s", edge.Target)
				return edge.Target
			}
		}
	}

	log.Printf("[FindNextNodeID] No next node found")
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
			// Send Interactive Button Message
			text := e.ReplaceVariables(waID, step.Content)

			// Build WhatsApp buttons (max 3)
			var buttons []whatsapp.ButtonObj

			for i, btn := range step.Buttons {
				if i >= 3 {
					break // WhatsApp limit
				}
				buttons = append(buttons, whatsapp.ButtonObj{
					Type: "reply",
					Reply: whatsapp.ReplyObj{
						ID:    fmt.Sprintf("btn_%d", i),
						Title: btn.Label,
					},
				})
			}

			e.WhatsAppClient.SendInteractiveButtons(waID, text, buttons)

		case "List":
			// Send Interactive List Message
			text := e.ReplaceVariables(waID, step.Content)
			buttonText := step.ButtonText
			if buttonText == "" {
				buttonText = "Select an option"
			}

			// Build WhatsApp list options (max 10)
			var options []whatsapp.RowObj
			for i, opt := range step.Options {
				if i >= 10 {
					break // WhatsApp limit
				}
				options = append(options, whatsapp.RowObj{
					ID:          fmt.Sprintf("opt_%d", i),
					Title:       opt.Title,
					Description: opt.Description,
				})
			}

			if len(options) > 0 {
				e.WhatsAppClient.SendInteractiveList(waID, text, buttonText, options)
			}

		case "Chatbot":
			// Jump to another flow or node
			if step.TargetFlowId != "" {
				log.Printf("[ExecuteNode] Chatbot step: Jumping to flow %s, node %s", step.TargetFlowId, step.TargetNodeId)

				// Get current session
				var session models.ConversationSession
				err := database.GormDB.Where("wa_id = ? AND status='active'", waID).First(&session).Error
				if err != nil {
					log.Printf("[ExecuteNode] Error getting session: %v", err)
					return err
				}

				// Update session to point to new flow
				err = database.GormDB.Model(&session).Updates(map[string]interface{}{
					"flow_id":      step.TargetFlowId,
					"current_node": step.TargetNodeId,
				}).Error
				if err != nil {
					log.Printf("[ExecuteNode] Error updating session: %v", err)
					return err
				}

				// Load target flow
				targetGraph, err := e.LoadGraph(step.TargetFlowId)
				if err != nil {
					log.Printf("[ExecuteNode] Error loading target graph: %v", err)
					return err
				}

				// Find target node (or start node if not specified)
				var targetNode *ReactFlowNode
				if step.TargetNodeId != "" {
					// Find specific node
					for _, n := range targetGraph.Nodes {
						if n.ID == step.TargetNodeId {
							targetNode = &n
							break
						}
					}
					if targetNode == nil {
						log.Printf("[ExecuteNode] Target node %s not found", step.TargetNodeId)
						e.WhatsAppClient.SendMessage(waID, "Error: Target node not found.")
						return fmt.Errorf("target node not found")
					}
				} else {
					// Find start node
					for _, n := range targetGraph.Nodes {
						if n.Data.IsStart {
							targetNode = &n
							break
						}
					}
					if targetNode == nil {
						log.Printf("[ExecuteNode] Start node not found in target flow")
						e.WhatsAppClient.SendMessage(waID, "Error: Start node not found in target flow.")
						return fmt.Errorf("start node not found")
					}
				}

				// Execute target node
				return e.ExecuteNode(waID, *targetNode, *targetGraph)
			}

		case "Image":
			e.WhatsAppClient.SendMessage(waID, "[Image] "+step.Content)

		case "Text Input", "Number Input", "Email Input":
			// Input steps don't send messages - they just wait for user input
			// The user should add a Text step before the Input step to ask the question
			// Do nothing here - just continue to the "wait" logic below
		}
	}

	// If this node ends with an Input Step, we STOP here.
	if len(node.Data.Steps) > 0 {
		lastStep := node.Data.Steps[len(node.Data.Steps)-1]
		if strings.Contains(lastStep.Type, "Input") {
			return nil // Stop and wait.
		}
		if lastStep.Type == "Quick Reply" || lastStep.Type == "List" {
			return nil // Stop and wait.
		}
	}

	// If NOT waiting for input, automatically move to next Node
	nextNodeID := e.FindNextNodeID(&node, graph.Edges, "")
	if nextNodeID != "" {
		var session models.ConversationSession
		database.GormDB.Where("wa_id = ? AND status='active'", waID).First(&session)
		database.GormDB.Model(&session).Update("current_node", nextNodeID)

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
		var session models.ConversationSession
		database.GormDB.Where("wa_id = ? AND status='active'", waID).First(&session)
		e.TerminateSessionByID(int(session.ID))
	}

	return nil
}

func (e *Engine) TerminateSession(waID string) {
	database.GormDB.Model(&models.ConversationSession{}).Where("wa_id = ? AND status = 'active'", waID).Update("status", "completed")
}

func (e *Engine) TerminateSessionByID(id int) {
	database.GormDB.Model(&models.ConversationSession{}).Where("id = ?", id).Update("status", "completed")
}

func (e *Engine) UpdateSessionContext(sessionID int, key, value string) {
	var session models.ConversationSession
	database.GormDB.First(&session, sessionID)

	var context map[string]string
	if session.Context == "" {
		context = make(map[string]string)
	} else {
		json.Unmarshal([]byte(session.Context), &context)
	}

	context[key] = value

	newContextJSON, _ := json.Marshal(context)
	database.GormDB.Model(&session).Update("context", string(newContextJSON))
}

func (e *Engine) GetContextInt(sessionID int, key string) int {
	var session models.ConversationSession
	database.GormDB.Select("context").First(&session, sessionID)

	if session.Context == "" {
		return 0
	}

	var context map[string]string
	if err := json.Unmarshal([]byte(session.Context), &context); err != nil {
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
	var contact models.Contact
	database.GormDB.Select("name").Where("wa_id = ?", waID).First(&contact)
	text = strings.ReplaceAll(text, "{{contact.name}}", contact.Name)
	text = strings.ReplaceAll(text, "{{contact.phone}}", waID)

	// 2. Get Session Context
	var session models.ConversationSession
	database.GormDB.Select("context").Where("wa_id = ? AND status='active'", waID).First(&session)

	if session.Context != "" {
		var context map[string]string
		json.Unmarshal([]byte(session.Context), &context)
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

// LoadGraph reconstructs a FlowGraphData from relational tables
func (e *Engine) LoadGraph(flowID string) (*FlowGraphData, error) {
	var nodes []models.FlowNode
	var edges []models.FlowEdge

	if err := database.GormDB.Where("flow_id = ?", flowID).Find(&nodes).Error; err != nil {
		return nil, err
	}
	if err := database.GormDB.Where("flow_id = ?", flowID).Find(&edges).Error; err != nil {
		return nil, err
	}

	graph := &FlowGraphData{
		Nodes: make([]ReactFlowNode, len(nodes)),
		Edges: make([]ReactFlowEdge, len(edges)),
	}

	for i, n := range nodes {
		var data ReactFlowNodeData
		if err := json.Unmarshal([]byte(n.Data), &data); err != nil {
			log.Printf("Error unmarshaling node data: %v", err)
		}
		graph.Nodes[i] = ReactFlowNode{
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
		graph.Edges[i] = ReactFlowEdge{
			ID:           e.EdgeID,
			Source:       e.Source,
			Target:       e.Target,
			SourceHandle: e.SourceHandle,
		}
	}

	return graph, nil
}
