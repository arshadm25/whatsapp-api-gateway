package database

import (
	"fmt"
	"log"

	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var GormDB *gorm.DB

func InitGorm(cfg *config.Config) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, cfg.DBSSLMode)

	var err error
	GormDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	log.Println("Connected to PostgreSQL successfully")

	// Auto Migration
	err = GormDB.AutoMigrate(
		&models.Message{},
		&models.Contact{},
		&models.Template{},
		&models.AutomationRule{},
		&models.ChatbotFlow{},
		&models.ScheduledMessage{},
		&models.ConversationSession{},
		&models.AutomationLog{},
		&models.Media{},
		&models.Flow{},
		&models.FlowNode{},
		&models.FlowEdge{},
		&models.SystemSetting{},
	)
	if err != nil {
		log.Fatalf("Failed to run auto-migration: %v", err)
	}

	log.Println("Database migration completed")
}

func SyncConfig(cfg *config.Config) {
	settings := []struct {
		Key   string
		Value *string
	}{
		{"VERIFY_TOKEN", &cfg.VerifyToken},
		{"WHATSAPP_TOKEN", &cfg.WhatsAppToken},
		{"PHONE_NUMBER_ID", &cfg.PhoneNumberID},
		{"WABA_ID", &cfg.WhatsAppBusinessAccountID},
	}

	for _, s := range settings {
		var setting models.SystemSetting
		if err := GormDB.Where("key = ?", s.Key).First(&setting).Error; err == nil {
			// Found in DB, update memory config
			if setting.Value != "" {
				*s.Value = setting.Value
			}
		} else {
			// Not found in DB, save current config to DB
			if *s.Value != "" {
				GormDB.Create(&models.SystemSetting{
					Key:   s.Key,
					Value: *s.Value,
				})
			}
		}
	}
	log.Println("System settings synchronized from database")
}
