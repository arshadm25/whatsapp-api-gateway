package database

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dbPath string) {
	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	createTableSQL := `CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		wa_id TEXT,
		sender TEXT,
		content TEXT,
		type TEXT,
		status TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	_, err = DB.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create table messages: %v", err)
	}

	createContactsSQL := `CREATE TABLE IF NOT EXISTS contacts (
		wa_id TEXT PRIMARY KEY,
		name TEXT,
		profile_pic_url TEXT,
		tags TEXT, 
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(createContactsSQL); err != nil {
		log.Fatalf("Failed to create table contacts: %v", err)
	}

	createTemplatesSQL := `CREATE TABLE IF NOT EXISTS templates (
		id TEXT PRIMARY KEY,
		name TEXT,
		language TEXT,
		category TEXT,
		status TEXT,
		components TEXT
	);`
	if _, err := DB.Exec(createTemplatesSQL); err != nil {
		log.Fatalf("Failed to create table templates: %v", err)
	}

	// Automation Rules Table
	createAutomationRulesSQL := `CREATE TABLE IF NOT EXISTS automation_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		enabled BOOLEAN DEFAULT 1,
		priority INTEGER DEFAULT 0,
		conditions TEXT,
		actions TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(createAutomationRulesSQL); err != nil {
		log.Fatalf("Failed to create table automation_rules: %v", err)
	}

	// Chatbot Flows Table
	createChatbotFlowsSQL := `CREATE TABLE IF NOT EXISTS chatbot_flows (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT,
		trigger_keywords TEXT,
		enabled BOOLEAN DEFAULT 1,
		nodes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(createChatbotFlowsSQL); err != nil {
		log.Fatalf("Failed to create table chatbot_flows: %v", err)
	}

	// Scheduled Messages Table
	createScheduledMessagesSQL := `CREATE TABLE IF NOT EXISTS scheduled_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		recipient_wa_id TEXT,
		message_content TEXT,
		template_id TEXT,
		scheduled_time DATETIME NOT NULL,
		recurrence TEXT,
		status TEXT DEFAULT 'pending',
		sent_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(createScheduledMessagesSQL); err != nil {
		log.Fatalf("Failed to create table scheduled_messages: %v", err)
	}

	// Conversation Sessions Table
	createConversationSessionsSQL := `CREATE TABLE IF NOT EXISTS conversation_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		wa_id TEXT NOT NULL,
		flow_id INTEGER,
		current_node TEXT,
		context TEXT,
		status TEXT DEFAULT 'active',
		started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (flow_id) REFERENCES chatbot_flows(id)
	);`
	if _, err := DB.Exec(createConversationSessionsSQL); err != nil {
		log.Fatalf("Failed to create table conversation_sessions: %v", err)
	}

	// Automation Logs Table
	createAutomationLogsSQL := `CREATE TABLE IF NOT EXISTS automation_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		rule_id INTEGER,
		wa_id TEXT,
		trigger_type TEXT,
		action_taken TEXT,
		success BOOLEAN,
		error_message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(createAutomationLogsSQL); err != nil {
		log.Fatalf("Failed to create table automation_logs: %v", err)
	}

	// Media Table
	createMediaSQL := `CREATE TABLE IF NOT EXISTS media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		media_id TEXT NOT NULL UNIQUE,
		filename TEXT,
		mime_type TEXT,
		file_size INTEGER,
		uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(createMediaSQL); err != nil {
		log.Fatalf("Failed to create table media: %v", err)
	}

	// Flows Table (Local state for WhatsApp Flows)
	createFlowsSQL := `CREATE TABLE IF NOT EXISTS flows (
		id TEXT PRIMARY KEY,
		name TEXT,
		status TEXT,
		graph_data TEXT, -- ReactFlow nodes/edges
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	if _, err := DB.Exec(createFlowsSQL); err != nil {
		log.Fatalf("Failed to create table flows: %v", err)
	}

	log.Println("Database initialized successfully (messages, contacts, templates, automation, media)")
}
