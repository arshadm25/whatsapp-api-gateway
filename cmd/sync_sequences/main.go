package main

import (
	"log"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
)

func main() {
	cfg := config.LoadConfig()
	database.InitGorm(cfg)
	db := database.GormDB

	tables := []string{
		"messages",
		"automation_rules",
		"chatbot_flows",
		"scheduled_messages",
		"conversation_sessions",
		"automation_logs",
		"media",
		"flow_nodes",
		"flow_edges",
	}

	log.Println("Syncing PostgreSQL sequences...")

	for _, table := range tables {
		query := "SELECT setval(pg_get_serial_sequence('" + table + "', 'id'), coalesce(max(id), 0) + 1, false) FROM " + table
		if err := db.Exec(query).Error; err != nil {
			log.Printf("Error syncing sequence for %s: %v", table, err)
		} else {
			log.Printf("Successfully synced sequence for %s", table)
		}
	}

	log.Println("DONE!")
}
