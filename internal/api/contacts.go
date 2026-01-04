package api

import (
	"fmt"
	"net/http"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/internal/models"

	"github.com/gin-gonic/gin"
)

type ContactHandler struct{}

func NewContactHandler() *ContactHandler {
	return &ContactHandler{}
}

func (h *ContactHandler) GetContacts(c *gin.Context) {
	var contacts []models.Contact
	if err := database.GormDB.Order("created_at desc").Find(&contacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, contacts)
}

type UpdateContactRequest struct {
	Name string `json:"name"`
	Tags string `json:"tags"`
}

func (h *ContactHandler) UpdateContact(c *gin.Context) {
	waID := c.Param("waId")
	var req UpdateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := database.GormDB.Model(&models.Contact{}).Where("wa_id = ?", waID).Updates(models.Contact{
		Name: req.Name,
		Tags: req.Tags,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update contact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Contact updated"})
}

type CreateContactRequest struct {
	WaID string `json:"wa_id" binding:"required"`
	Name string `json:"name"`
	Tags string `json:"tags"`
}

func (h *ContactHandler) CreateContact(c *gin.Context) {
	var req CreateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contact := models.Contact{
		WaID: req.WaID,
		Name: req.Name,
		Tags: req.Tags,
	}

	// Use Save for upsert
	if err := database.GormDB.Save(&contact).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create contact"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Contact created", "wa_id": req.WaID})
}

func (h *ContactHandler) DeleteContact(c *gin.Context) {
	waID := c.Param("waId")

	result := database.GormDB.Where("wa_id = ?", waID).Delete(&models.Contact{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete contact"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Contact deleted"})
}

func (h *ContactHandler) ExportContacts(c *gin.Context) {
	var contacts []models.Contact
	if err := database.GormDB.Order("created_at desc").Find(&contacts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Build CSV content
	csv := "WhatsApp ID,Name,Tags,Created At\n"
	for _, contact := range contacts {
		csv += fmt.Sprintf("%s,%s,%s,%s\n", contact.WaID, contact.Name, contact.Tags, contact.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=contacts.csv")
	c.String(http.StatusOK, csv)
}
