package api

import (
	"database/sql"
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
