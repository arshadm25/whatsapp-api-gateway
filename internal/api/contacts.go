package api

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"whatsapp-gateway/internal/database"
	"whatsapp-gateway/pkg/models"

	"github.com/gin-gonic/gin"
)

type ContactHandler struct{}

func NewContactHandler() *ContactHandler {
	return &ContactHandler{}
}

func (h *ContactHandler) GetContacts(c *gin.Context) {
	rows, err := database.DB.Query("SELECT wa_id, name, profile_pic_url, tags, created_at FROM contacts ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var contacts []models.Contact
	for rows.Next() {
		var contact models.Contact
		var profilePicURL sql.NullString
		var tags sql.NullString
		if err := rows.Scan(&contact.WaID, &contact.Name, &profilePicURL, &tags, &contact.CreatedAt); err != nil {
			log.Printf("Error scanning contact: %v", err)
			continue
		}
		if profilePicURL.Valid {
			contact.ProfilePicURL = profilePicURL.String
		}
		if tags.Valid {
			contact.Tags = tags.String
		}
		contacts = append(contacts, contact)
	}

	// Return empty array instead of null
	if contacts == nil {
		contacts = []models.Contact{}
	}

	c.JSON(http.StatusOK, contacts)
}

type UpdateContactRequest struct {
	Name string `json:"name"`
	Tags string `json:"tags"` // Expecting JSON string for simplicity or specific tag logic
}

func (h *ContactHandler) UpdateContact(c *gin.Context) {
	waID := c.Param("waId")
	var req UpdateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := database.DB.Exec("UPDATE contacts SET name = ?, tags = ? WHERE wa_id = ?", req.Name, req.Tags, waID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update contact"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Contact updated"})
}

// CreateContactRequest for adding new contacts
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

	// Use UPSERT to avoid duplicates
	_, err := database.DB.Exec(`INSERT INTO contacts(wa_id, name, tags) VALUES(?, ?, ?) 
		ON CONFLICT(wa_id) DO UPDATE SET name=excluded.name, tags=excluded.tags`,
		req.WaID, req.Name, req.Tags)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create contact"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Contact created", "wa_id": req.WaID})
}

func (h *ContactHandler) DeleteContact(c *gin.Context) {
	waID := c.Param("waId")

	result, err := database.DB.Exec("DELETE FROM contacts WHERE wa_id = ?", waID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete contact"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Contact not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "Contact deleted"})
}

func (h *ContactHandler) ExportContacts(c *gin.Context) {
	rows, err := database.DB.Query("SELECT wa_id, name, profile_pic_url, tags, created_at FROM contacts ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	// Build CSV content
	csv := "WhatsApp ID,Name,Tags,Created At\n"
	for rows.Next() {
		var waID, name, createdAt string
		var profilePicURL, tags sql.NullString
		if err := rows.Scan(&waID, &name, &profilePicURL, &tags, &createdAt); err != nil {
			continue
		}
		tagsStr := ""
		if tags.Valid {
			tagsStr = tags.String
		}
		csv += fmt.Sprintf("%s,%s,%s,%s\n", waID, name, tagsStr, createdAt)
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=contacts.csv")
	c.String(http.StatusOK, csv)
}
