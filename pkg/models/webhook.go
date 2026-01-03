package models

// WebhookPayload represents the incoming JSON payload from WhatsApp
type WebhookPayload struct {
	Object string `json:"object"`
	Entry  []struct {
		ID      string `json:"id"`
		Changes []struct {
			Value struct {
				MessagingProduct string `json:"messaging_product"`
				Metadata         struct {
					DisplayPhoneNumber string `json:"display_phone_number"`
					PhoneNumberID      string `json:"phone_number_id"`
				} `json:"metadata"`
				Messages []struct {
					From      string `json:"from"`
					ID        string `json:"id"`
					Timestamp string `json:"timestamp"`
					Text      struct {
						Body string `json:"body"`
					} `json:"text,omitempty"`
					Image    *MediaMessage `json:"image,omitempty"`
					Video    *MediaMessage `json:"video,omitempty"`
					Audio    *MediaMessage `json:"audio,omitempty"`
					Document *MediaMessage `json:"document,omitempty"`
					Type     string        `json:"type"`
				} `json:"messages,omitempty"`
				Statuses []struct {
					ID          string `json:"id"`
					Status      string `json:"status"`
					Timestamp   string `json:"timestamp"`
					RecipientId string `json:"recipient_id"`
				} `json:"statuses,omitempty"`
			} `json:"value"`
			Field string `json:"field"`
		} `json:"changes"`
	} `json:"entry"`
}

// MediaMessage represents a media attachment in a WhatsApp message
type MediaMessage struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
}

// Message represents a flattened message structure for our DB/Dashboard
type Message struct {
	ID        int    `json:"id"`
	WaID      string `json:"wa_id"`
	Sender    string `json:"sender"`
	Content   string `json:"content"`
	Type      string `json:"type"`
	Status    string `json:"status"` // received, sent
	CreatedAt string `json:"created_at"`
}
