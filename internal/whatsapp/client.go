package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"whatsapp-gateway/internal/config"
	"whatsapp-gateway/internal/database"
)

type Client struct {
	Config *config.Config
}

func NewClient(cfg *config.Config) *Client {
	return &Client{Config: cfg}
}

// --- Message Structures ---

type GenericMessage struct {
	MessagingProduct string          `json:"messaging_product"`
	To               string          `json:"to"`
	Type             string          `json:"type"`
	RecipientType    string          `json:"recipient_type,omitempty"`
	Text             *TextObj        `json:"text,omitempty"`
	Image            *MediaObj       `json:"image,omitempty"`
	Video            *MediaObj       `json:"video,omitempty"`
	Audio            *MediaObj       `json:"audio,omitempty"`
	Document         *MediaObj       `json:"document,omitempty"`
	Sticker          *MediaObj       `json:"sticker,omitempty"`
	Location         *LocationObj    `json:"location,omitempty"`
	Template         *TemplateObj    `json:"template,omitempty"`
	Interactive      *InteractiveObj `json:"interactive,omitempty"`
}

type TextObj struct {
	Body       string `json:"body"`
	PreviewUrl bool   `json:"preview_url,omitempty"`
}

type MediaObj struct {
	ID       string `json:"id,omitempty"`
	Link     string `json:"link,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"` // For documents
}

type LocationObj struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

type TemplateObj struct {
	Name       string         `json:"name"`
	Language   LanguageObj    `json:"language"`
	Components []ComponentObj `json:"components,omitempty"`
}

type LanguageObj struct {
	Code string `json:"code"`
}

type ComponentObj struct {
	Type       string         `json:"type"`
	SubType    string         `json:"sub_type,omitempty"`
	Parameters []ParameterObj `json:"parameters"`
	Index      string         `json:"index,omitempty"` // For buttons
}

type ParameterObj struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	Currency *CurrencyObj `json:"currency,omitempty"`
	DateTime *DateTimeObj `json:"date_time,omitempty"`
	Image    *MediaObj    `json:"image,omitempty"`
	Video    *MediaObj    `json:"video,omitempty"`
	Document *MediaObj    `json:"document,omitempty"`
}

type CurrencyObj struct {
	FallbackValue string `json:"fallback_value"`
	Code          string `json:"code"`
	Amount1000    int    `json:"amount_1000"`
}

type DateTimeObj struct {
	FallbackValue string `json:"fallback_value"`
}

type InteractiveObj struct {
	Type   string     `json:"type"`
	Header *HeaderObj `json:"header,omitempty"`
	Body   BodyObj    `json:"body"`
	Footer *FooterObj `json:"footer,omitempty"`
	Action ActionObj  `json:"action"`
}

type HeaderObj struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	Video    *MediaObj `json:"video,omitempty"`
	Image    *MediaObj `json:"image,omitempty"`
	Document *MediaObj `json:"document,omitempty"`
}

type BodyObj struct {
	Text string `json:"text"`
}

type FooterObj struct {
	Text string `json:"text"`
}

type ActionObj struct {
	Button            string       `json:"button,omitempty"`
	Buttons           []ButtonObj  `json:"buttons,omitempty"`
	Sections          []SectionObj `json:"sections,omitempty"`
	CatalogID         string       `json:"catalog_id,omitempty"`
	ProductRetailerID string       `json:"product_retailer_id,omitempty"`
	// Flow specific fields
	Name       string      `json:"name,omitempty"` // for flow_cta
	Parameters *FlowParams `json:"parameters,omitempty"`
}

type FlowParams struct {
	FlowMessageVersion string             `json:"flow_message_version"`
	FlowToken          string             `json:"flow_token"`
	FlowID             string             `json:"flow_id,omitempty"`
	FlowName           string             `json:"flow_name,omitempty"`
	FlowCTA            string             `json:"flow_cta"`
	FlowAction         string             `json:"flow_action,omitempty"` // navigate or data_exchange
	FlowActionPayload  *FlowActionPayload `json:"flow_action_payload,omitempty"`
}

type FlowActionPayload struct {
	Screen string      `json:"screen"`
	Data   interface{} `json:"data,omitempty"`
}

type ButtonObj struct {
	Type  string   `json:"type"`
	Reply ReplyObj `json:"reply"`
}

type ReplyObj struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type SectionObj struct {
	Title        string        `json:"title,omitempty"`
	ProductItems []ProductItem `json:"product_items,omitempty"`
	Rows         []RowObj      `json:"rows,omitempty"`
}

type ProductItem struct {
	ProductRetailerID string `json:"product_retailer_id"`
}

type RowObj struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// --- Helper Functions ---

func (c *Client) sendRequest(method, url string, body interface{}, headers map[string]string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Config.WhatsAppToken)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// Default to JSON if not specified (unless body is nil/multipart which handles itself usually, but here we handled json marshal)
	if req.Header.Get("Content-Type") == "" && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("API error: %s - %s", resp.Status, string(respBody))
	}

	return respBody, nil
}

// --- Messaging Methods ---

func (c *Client) SendRawMessage(msg GenericMessage) error {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/messages", c.Config.PhoneNumberID)
	_, err := c.sendRequest("POST", url, msg, nil)

	// Access the body to log it
	content := ""
	if msg.Text != nil {
		content = msg.Text.Body
	} else if msg.Template != nil {
		content = "Template: " + msg.Template.Name
	} else {
		content = fmt.Sprintf("%s message", msg.Type)
	}

	// Log to DB (Fire and forget or simple log)
	// Store the recipient phone number in 'sender' field so we can group conversations properly
	go func() {
		stmt, err := database.DB.Prepare("INSERT INTO messages(wa_id, sender, content, type, status) VALUES(?, ?, ?, ?, ?)")
		if err == nil {
			stmt.Exec("outgoing-"+msg.To, msg.To, content, msg.Type, "sent")
			stmt.Close()
		}
	}()

	return err
}

func (c *Client) SendMessage(to, body string) error {
	msg := GenericMessage{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
		Text: &TextObj{
			Body: body,
		},
	}
	return c.SendRawMessage(msg)
}

func (c *Client) SendTemplateMessage(to, templateName, languageCode string) error {
	msg := GenericMessage{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "template",
		Template: &TemplateObj{
			Name: templateName,
			Language: LanguageObj{
				Code: languageCode,
			},
		},
	}
	return c.SendRawMessage(msg)
}

func (c *Client) SendImageMessage(to, imageUrl, caption string) error {
	msg := GenericMessage{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "image",
		Image: &MediaObj{
			Link:    imageUrl,
			Caption: caption,
		},
	}
	return c.SendRawMessage(msg)
}

// --- Media Methods ---

type MediaResponse struct {
	ID string `json:"id"`
}

func (c *Client) UploadMedia(fileData []byte, mimeType, filename string) (*MediaResponse, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/media", c.Config.PhoneNumberID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	part.Write(fileData)

	writer.WriteField("messaging_product", "whatsapp")
	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Config.WhatsAppToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upload failed: %s - %s", resp.Status, string(respBody))
	}

	var mediaResp MediaResponse
	if err := json.Unmarshal(respBody, &mediaResp); err != nil {
		return nil, err
	}

	return &mediaResp, nil
}

func (c *Client) RetrieveMediaURL(mediaID string) (string, error) {
	// First get the media object URL
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s", mediaID)
	resp, err := c.sendRequest("GET", url, nil, nil)
	if err != nil {
		return "", err
	}

	var obj struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(resp, &obj); err != nil {
		return "", err
	}

	// If you need to actually download the bytes, you would make another request to obj.URL
	// with the Authorization header.
	return obj.URL, nil
}

func (c *Client) DeleteMedia(mediaID string) error {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s", mediaID)
	_, err := c.sendRequest("DELETE", url, nil, nil)
	return err
}

// --- Template Management Methods ---

func (c *Client) GetTemplates() (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/message_templates", c.Config.WhatsAppBusinessAccountID)
	// We return raw interface{} or map[string]interface{} to just pass it through
	// or we could define complex template structs.
	resp, err := c.sendRequest("GET", url, nil, nil)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) CreateTemplate(templateData interface{}) (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/message_templates", c.Config.WhatsAppBusinessAccountID)
	resp, err := c.sendRequest("POST", url, templateData, nil)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) DeleteTemplate(templateName string) error {
	// Deleting by name usually requires filtering or a specific ID, but the Management API often uses parameters.
	// Actually, DELETE https://graph.facebook.com/v19.0/{waba_id}/message_templates?name={name}
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/message_templates?name=%s", c.Config.WhatsAppBusinessAccountID, templateName)
	_, err := c.sendRequest("DELETE", url, nil, nil)
	return err
}

// --- Flow Management Methods ---

func (c *Client) GetFlows() (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/flows", c.Config.WhatsAppBusinessAccountID)
	resp, err := c.sendRequest("GET", url, nil, nil)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) GetFlow(flowID string) (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s?fields=id,name,categories,preview,status,validation_errors,json_version,data_api_version,data_channel_uri,health_status", flowID)
	resp, err := c.sendRequest("GET", url, nil, nil)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) CreateFlow(name string, categories []string, cloneFlowID string) (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/flows", c.Config.WhatsAppBusinessAccountID)

	req := map[string]interface{}{
		"name":       name,
		"categories": categories,
	}
	if cloneFlowID != "" {
		req["clone_flow_id"] = cloneFlowID
	}

	resp, err := c.sendRequest("POST", url, req, nil)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) UpdateFlowMetadata(flowID, name string, categories []string) (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s", flowID)
	req := map[string]interface{}{}
	if name != "" {
		req["name"] = name
	}
	if len(categories) > 0 {
		req["categories"] = categories
	}

	resp, err := c.sendRequest("POST", url, req, nil)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) PublishFlow(flowID string) (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/publish", flowID)
	resp, err := c.sendRequest("POST", url, nil, nil)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) DeleteFlow(flowID string) (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s", flowID)
	resp, err := c.sendRequest("DELETE", url, nil, nil)
	if err != nil {
		return nil, err
	}
	var result interface{}
	err = json.Unmarshal(resp, &result)
	return result, err
}

func (c *Client) UploadFlowJSON(flowID string, fileData []byte) (interface{}, error) {
	url := fmt.Sprintf("https://graph.facebook.com/v19.0/%s/assets", flowID)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "flow.json")
	if err != nil {
		return nil, err
	}
	part.Write(fileData)

	writer.Close()

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Config.WhatsAppToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("upload flow failed: %s - %s", resp.Status, string(respBody))
	}

	var result interface{}
	err = json.Unmarshal(respBody, &result)
	return result, err
}
