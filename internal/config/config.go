package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                      string
	VerifyToken               string
	WhatsAppToken             string
	PhoneNumberID             string
	WhatsAppBusinessAccountID string
	DBPath                    string
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file")
	}

	return &Config{
		Port:                      getEnv("PORT", "8080"),
		VerifyToken:               getEnv("VERIFY_TOKEN", ""),
		WhatsAppToken:             getEnv("WHATSAPP_TOKEN", ""),
		PhoneNumberID:             getEnv("PHONE_NUMBER_ID", ""),
		WhatsAppBusinessAccountID: getEnv("WABA_ID", ""),
		DBPath:                    getEnv("DB_PATH", "./whatsapp.db"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
