package models

import "time"

// Media represents an uploaded media file
type Media struct {
	ID         int       `json:"id"`
	MediaID    string    `json:"media_id"`
	Filename   string    `json:"filename"`
	MimeType   string    `json:"mime_type"`
	FileSize   int64     `json:"file_size"`
	UploadedAt time.Time `json:"uploaded_at"`
}
