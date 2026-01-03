package models

// Contact represents a person who has messaged the business
type Contact struct {
	WaID          string `json:"wa_id"`
	Name          string `json:"name"`
	ProfilePicURL string `json:"profile_pic_url"`
	Tags          string `json:"tags"` // JSON array string
	CreatedAt     string `json:"created_at"`
}

// Template represents a WhatsApp Message Template
type Template struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Language   string `json:"language"`
	Category   string `json:"category"`
	Status     string `json:"status"`     // APPROVED, REJECTED, PENDING
	Components string `json:"components"` // JSON string of components
}
