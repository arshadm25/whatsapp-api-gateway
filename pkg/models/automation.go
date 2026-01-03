package models

// AutomationRule represents an automation rule in the system
type AutomationRule struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"` // auto_reply, keyword_trigger, chatbot_flow
	Enabled    bool   `json:"enabled"`
	Priority   int    `json:"priority"`
	Conditions string `json:"conditions"` // JSON
	Actions    string `json:"actions"`    // JSON
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// ChatbotFlow represents a chatbot conversation flow
type ChatbotFlow struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	TriggerKeywords string `json:"trigger_keywords"` // JSON array
	Enabled         bool   `json:"enabled"`
	Nodes           string `json:"nodes"` // JSON flow definition
	CreatedAt       string `json:"created_at"`
}

// ScheduledMessage represents a scheduled message
type ScheduledMessage struct {
	ID             int    `json:"id"`
	RecipientWaID  string `json:"recipient_wa_id"`
	MessageContent string `json:"message_content"`
	TemplateID     string `json:"template_id"`
	ScheduledTime  string `json:"scheduled_time"`
	Recurrence     string `json:"recurrence"` // once, daily, weekly, monthly
	Status         string `json:"status"`     // pending, sent, failed, cancelled
	SentAt         string `json:"sent_at"`
	CreatedAt      string `json:"created_at"`
}

// ConversationSession represents an active chatbot conversation
type ConversationSession struct {
	ID          int    `json:"id"`
	WaID        string `json:"wa_id"`
	FlowID      int    `json:"flow_id"`
	CurrentNode string `json:"current_node"`
	Context     string `json:"context"` // JSON
	Status      string `json:"status"`  // active, completed, abandoned
	StartedAt   string `json:"started_at"`
	UpdatedAt   string `json:"updated_at"`
}

// AutomationLog represents a log entry for automation actions
type AutomationLog struct {
	ID           int    `json:"id"`
	RuleID       int    `json:"rule_id"`
	WaID         string `json:"wa_id"`
	TriggerType  string `json:"trigger_type"`
	ActionTaken  string `json:"action_taken"`
	Success      bool   `json:"success"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
}
