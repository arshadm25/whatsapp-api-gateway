package main

import (
	"log"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	cfg := config.LoadConfig()

	// 1. Connect to SQLite (Source)
	sqliteDB, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}
	log.Printf("Connected to SQLite at %s", cfg.DBPath)

	// 2. Connect to PostgreSQL (Destination)
	database.InitGorm(cfg)
	pgDB := database.GormDB

	log.Println("Starting data migration...")

	// Migration Helper
	migrateTable := func(tableName string, source interface{}, dest interface{}) {
		log.Printf("Migrating table: %s", tableName)

		// Read from SQLite
		if err := sqliteDB.Find(source).Error; err != nil {
			log.Printf("Error reading %s from SQLite: %v", tableName, err)
			return
		}

		// Write to Postgres
		// Using transaction for safety
		err := pgDB.Transaction(func(tx *gorm.DB) error {
			// Optional: Clear destination table if you want a clean migration
			// tx.Exec("TRUNCATE TABLE " + tableName + " RESTART IDENTITY CASCADE")

			// We use map to avoid GORM logic that might skip IDs or something
			// Actually, if we pass the slice of structs, GORM should handle it if IDs are non-zero.

			// For large tables, we might want to batch
			// But since this is a one-time migration for a local dev, Find then Create should be fine.

			// Use a generic interface to check length
			// source is a pointer to a slice
			return tx.Create(source).Error
		})

		if err != nil {
			log.Printf("Error writing %s to Postgres: %v", tableName, err)
		} else {
			log.Printf("Successfully migrated %s", tableName)
		}
	}

	// Migrate in order (handle dependencies if any, though most are independent here)

	// 1. Contacts
	var contacts []models.Contact
	migrateTable("contacts", &contacts, &models.Contact{})

	// 2. Messages
	var messages []models.Message
	migrateTable("messages", &messages, &models.Message{})

	// 3. Templates
	var templates []models.Template
	migrateTable("templates", &templates, &models.Template{})

	// 4. Automation Rules
	var rules []models.AutomationRule
	migrateTable("automation_rules", &rules, &models.AutomationRule{})

	// 5. Automation Logs
	var logs []models.AutomationLog
	migrateTable("automation_logs", &logs, &models.AutomationLog{})

	// 6. Flows
	var flows []models.Flow
	migrateTable("flows", &flows, &models.Flow{})

	// 7. Conversation Sessions
	var sessions []models.ConversationSession
	migrateTable("conversation_sessions", &sessions, &models.ConversationSession{})

	// 8. Media
	var media []models.Media
	migrateTable("media", &media, &models.Media{})

	// 9. Scheduled Messages
	var scheduled []models.ScheduledMessage
	migrateTable("scheduled_messages", &scheduled, &models.ScheduledMessage{})

	log.Println("Migration completed!")
}
