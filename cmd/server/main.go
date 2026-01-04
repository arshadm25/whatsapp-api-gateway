package main

import (
	"log"
	"whatsapp-gateway/internal/api"
	"whatsapp-gateway/internal/automation"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/webhook"
	"whatsapp-gateway/internal/whatsapp"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()
	database.InitDB(cfg.DBPath)

	r := gin.Default()

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	whatsappClient := whatsapp.NewClient(cfg)
	automationEngine := automation.NewEngine(whatsappClient)
	webhookHandler := webhook.NewHandler(cfg, automationEngine)
	dashboardHandler := api.NewDashboardHandler(whatsappClient)
	contactHandler := api.NewContactHandler()
	broadcastHandler := api.NewBroadcastHandler(whatsappClient, cfg)
	automationHandler := api.NewAutomationHandler()
	whatsappHandler := api.NewWhatsAppHandler(whatsappClient)

	// Webhook Routes
	r.GET("/webhook", webhookHandler.VerifyWebhook)
	r.POST("/webhook", webhookHandler.HandleMessage)

	// Dashboard API Routes
	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/messages", dashboardHandler.GetMessages)
		apiGroup.POST("/send", dashboardHandler.SendMessage)

		// CRM Routes
		apiGroup.GET("/contacts", contactHandler.GetContacts)
		apiGroup.POST("/contacts", contactHandler.CreateContact)
		apiGroup.PUT("/contacts/:waId", contactHandler.UpdateContact)
		apiGroup.DELETE("/contacts/:waId", contactHandler.DeleteContact)
		apiGroup.GET("/contacts/export", contactHandler.ExportContacts)

		// Broadcast Routes
		apiGroup.GET("/templates", broadcastHandler.GetTemplates)
		apiGroup.GET("/templates/meta", broadcastHandler.GetTemplatesFromMeta)
		apiGroup.POST("/templates/sync", broadcastHandler.SyncTemplates)
		apiGroup.POST("/broadcast", broadcastHandler.SendBroadcast)

		// Automation Routes
		apiGroup.GET("/automation/rules", automationHandler.GetRules)
		apiGroup.POST("/automation/rules", automationHandler.CreateRule)
		apiGroup.PUT("/automation/rules/:id", automationHandler.UpdateRule)
		apiGroup.DELETE("/automation/rules/:id", automationHandler.DeleteRule)
		apiGroup.POST("/automation/rules/:id/toggle", automationHandler.ToggleRule)
		apiGroup.GET("/automation/logs", automationHandler.GetLogs)
		apiGroup.GET("/automation/analytics", automationHandler.GetAnalytics)

		// WhatsApp Direct API Routes
		whatsappGroup := apiGroup.Group("/whatsapp")
		{
			whatsappGroup.POST("/send", whatsappHandler.SendMessage)
			whatsappGroup.POST("/media", whatsappHandler.UploadMedia)
			whatsappGroup.GET("/media", whatsappHandler.ListMedia)
			whatsappGroup.GET("/media/:id", whatsappHandler.RetrieveMediaURL)
			whatsappGroup.GET("/media/:id/proxy", whatsappHandler.DownloadMediaProxy)
			whatsappGroup.DELETE("/media/:id", whatsappHandler.DeleteMedia)
			whatsappGroup.GET("/templates", whatsappHandler.GetTemplates)
			whatsappGroup.POST("/templates", whatsappHandler.CreateTemplate)
			whatsappGroup.DELETE("/templates", whatsappHandler.DeleteTemplate)

			// Local Flow Routes
			whatsappGroup.GET("/flows/local", whatsappHandler.GetLocalFlows)
			whatsappGroup.POST("/flows/local", whatsappHandler.SaveLocalFlow)
			whatsappGroup.GET("/flows/local/:id", whatsappHandler.GetLocalFlow)
			whatsappGroup.DELETE("/flows/local/:id", whatsappHandler.DeleteLocalFlow)

			// WhatsApp Flow Routes
			whatsappGroup.GET("/flows", whatsappHandler.GetFlows)
			whatsappGroup.POST("/flows", whatsappHandler.CreateFlow)
			whatsappGroup.GET("/flows/:id", whatsappHandler.GetFlow)
			whatsappGroup.POST("/flows/:id", whatsappHandler.UpdateFlowMetadata)
			whatsappGroup.POST("/flows/:id/assets", whatsappHandler.UploadFlowJSON)
			whatsappGroup.POST("/flows/:id/publish", whatsappHandler.PublishFlow)
			whatsappGroup.DELETE("/flows/:id", whatsappHandler.DeleteFlow)
		}
	}

	log.Printf("Server starting on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
