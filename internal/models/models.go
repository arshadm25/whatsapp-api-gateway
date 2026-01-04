package models

import (
	"time"
)

// Message represents a WhatsApp message
type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	WaID      string    `gorm:"index;not null" json:"wa_id"`
	Sender    string    `gorm:"not null" json:"sender"`
	Content   string    `gorm:"type:text" json:"content"`
	Type      string    `gorm:"type:varchar(50)" json:"type"`
	Status    string    `gorm:"type:varchar(20)" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Message) TableName() string {
	return "messages"
}

// Contact represents a WhatsApp contact
type Contact struct {
	WaID          string    `gorm:"primaryKey" json:"wa_id"` // WhatsApp ID (phone number)
	Name          string    `gorm:"type:varchar(255)" json:"name"`
	ProfilePicURL string    `gorm:"type:text" json:"profile_pic_url"`
	Tags          string    `gorm:"type:text" json:"tags"` // Comma separated tags
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Contact) TableName() string {
	return "contacts"
}

// Template represents a WhatsApp message template
type Template struct {
	ID         string `gorm:"primaryKey" json:"id"`
	Name       string `gorm:"type:varchar(255)" json:"name"`
	Language   string `gorm:"type:varchar(50)" json:"language"`
	Category   string `gorm:"type:varchar(100)" json:"category"`
	Status     string `gorm:"type:varchar(50)" json:"status"`
	Components string `gorm:"type:text" json:"components"` // JSON components
}

func (Template) TableName() string {
	return "templates"
}

// AutomationRule represents an automation trigger/action rule
type AutomationRule struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"type:varchar(255);not null" json:"name"`
	Type       string    `gorm:"type:varchar(50);not null" json:"type"`
	Enabled    bool      `gorm:"default:true" json:"enabled"`
	Priority   int       `gorm:"default:0" json:"priority"`
	Conditions string    `gorm:"type:text" json:"conditions"` // JSON conditions
	Actions    string    `gorm:"type:text" json:"actions"`    // JSON actions
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AutomationRule) TableName() string {
	return "automation_rules"
}

// ChatbotFlow represents a legacy chatbot flow structure (if still used)
type ChatbotFlow struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"type:varchar(255);not null" json:"name"`
	Description     string    `gorm:"type:text" json:"description"`
	TriggerKeywords string    `gorm:"type:text" json:"trigger_keywords"`
	Enabled         bool      `gorm:"default:true" json:"enabled"`
	Nodes           string    `gorm:"type:text" json:"nodes"` // JSON nodes
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ChatbotFlow) TableName() string {
	return "chatbot_flows"
}

// ScheduledMessage represents a message to be sent at a future time
type ScheduledMessage struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	RecipientWaID  string     `gorm:"type:varchar(50)" json:"recipient_wa_id"`
	MessageContent string     `gorm:"type:text" json:"message_content"`
	TemplateID     string     `gorm:"type:varchar(255)" json:"template_id"`
	ScheduledTime  time.Time  `gorm:"not null" json:"scheduled_time"`
	Recurrence     string     `gorm:"type:varchar(50)" json:"recurrence"`
	Status         string     `gorm:"type:varchar(20);default:'pending'" json:"status"`
	SentAt         *time.Time `json:"sent_at"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (ScheduledMessage) TableName() string {
	return "scheduled_messages"
}

// ConversationSession represents an active flow session for a user
type ConversationSession struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	WaID        string    `gorm:"type:varchar(50);not null;index" json:"wa_id"`
	FlowID      string    `gorm:"type:varchar(255)" json:"flow_id"`
	CurrentNode string    `gorm:"type:varchar(255)" json:"current_node"`
	Context     string    `gorm:"type:text" json:"context"` // JSON session variables
	Status      string    `gorm:"type:varchar(20);default:'active'" json:"status"`
	StartedAt   time.Time `gorm:"autoCreateTime" json:"started_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ConversationSession) TableName() string {
	return "conversation_sessions"
}

// AutomationLog represents a log entry for automation execution
type AutomationLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	RuleID       uint      `json:"rule_id"`
	WaID         string    `gorm:"type:varchar(50)" json:"wa_id"`
	TriggerType  string    `gorm:"type:varchar(50)" json:"trigger_type"`
	ActionTaken  string    `gorm:"type:text" json:"action_taken"`
	Success      bool      `json:"success"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AutomationLog) TableName() string {
	return "automation_logs"
}

// Media represents an uploaded media bit
type Media struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	MediaID    string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"media_id"`
	Filename   string    `gorm:"type:varchar(255)" json:"filename"`
	MimeType   string    `gorm:"type:varchar(100)" json:"mime_type"`
	FileSize   int64     `json:"file_size"`
	UploadedAt time.Time `gorm:"autoCreateTime" json:"uploaded_at"`
}

func (Media) TableName() string {
	return "media"
}

// Flow represents a WhatsApp Flow with ReactFlow graph data
type Flow struct {
	ID        string     `gorm:"primaryKey" json:"id"`
	Name      string     `gorm:"type:varchar(255)" json:"name"`
	Status    string     `gorm:"type:varchar(50)" json:"status"`
	Nodes     []FlowNode `gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE;" json:"nodes"`
	Edges     []FlowEdge `gorm:"foreignKey:FlowID;constraint:OnDelete:CASCADE;" json:"edges"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Flow) TableName() string {
	return "flows"
}

type FlowNode struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	FlowID    string    `gorm:"index;type:varchar(255)" json:"flow_id"`
	NodeID    string    `gorm:"type:varchar(255)" json:"node_id"` // ReactFlow node id
	Type      string    `gorm:"type:varchar(50)" json:"type"`
	PositionX float64   `json:"position_x"`
	PositionY float64   `json:"position_y"`
	Data      string    `gorm:"type:text" json:"data"` // Node data JSON (label, steps, isStart)
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (FlowNode) TableName() string {
	return "flow_nodes"
}

type FlowEdge struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	FlowID       string `gorm:"index;type:varchar(255)" json:"flow_id"`
	EdgeID       string `gorm:"type:varchar(255)" json:"edge_id"` // ReactFlow edge id
	Source       string `gorm:"type:varchar(255)" json:"source"`
	Target       string `gorm:"type:varchar(255)" json:"target"`
	SourceHandle string `gorm:"type:varchar(255)" json:"source_handle"`
}

func (FlowEdge) TableName() string {
	return "flow_edges"
}
