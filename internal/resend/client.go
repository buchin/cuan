package resend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "https://api.resend.com"
	DefaultTimeout = 10 * time.Second
)

var (
	ErrEmptyAPIKey     = errors.New("resend: API key cannot be empty")
	ErrInvalidAPIKey   = errors.New("resend: invalid API key or unauthorized")
	ErrEmptySender     = errors.New("resend: sender (from) email cannot be empty")
	ErrEmptyRecipients = errors.New("resend: recipient (to) email list cannot be empty")
	ErrEmptySubject    = errors.New("resend: email subject cannot be empty")
	ErrEmptyContent    = errors.New("resend: email html or text content cannot be empty")
)

// Client defines the interface for interacting with Resend email API.
type Client interface {
	SendEmail(ctx context.Context, req EmailRequest) (*EmailResponse, error)
	ValidateAPIKey(ctx context.Context) error
}

// HTTPClient represents the HTTP client implementation of Resend Client.
type HTTPClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// ClientOption configures an HTTPClient.
type ClientOption func(*HTTPClient)

// WithBaseURL overrides the API base URL (useful for testing and mock servers).
func WithBaseURL(url string) ClientOption {
	return func(c *HTTPClient) {
		if url != "" {
			c.baseURL = strings.TrimRight(url, "/")
		}
	}
}

// WithHTTPClient overrides the underlying http.Client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *HTTPClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// NewClient creates a new Resend API client with the given API key.
func NewClient(apiKey string, opts ...ClientOption) *HTTPClient {
	c := &HTTPClient{
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// EmailRequest represents a payload for sending an email via Resend.
type EmailRequest struct {
	IdempotencyKey string            `json:"-"`
	From           string            `json:"from"`
	To             []string          `json:"to"`
	Subject        string            `json:"subject"`
	HTML           string            `json:"html,omitempty"`
	Text           string            `json:"text,omitempty"`
	ReplyTo        string            `json:"reply_to,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
}

// EmailResponse represents the response from Resend's send email endpoint.
type EmailResponse struct {
	ID string `json:"id"`
}

// ResendAPIError represents error payload from Resend API.
type ResendAPIError struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Name       string `json:"name"`
}

func (e *ResendAPIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("resend api error (%d %s): %s", e.StatusCode, e.Name, e.Message)
	}
	return fmt.Sprintf("resend api error status %d", e.StatusCode)
}

// ValidateAPIKey checks if the client's API key is valid.
func (c *HTTPClient) ValidateAPIKey(ctx context.Context) error {
	if c.apiKey == "" {
		return ErrEmptyAPIKey
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api-keys", nil)
	if err != nil {
		return fmt.Errorf("create validation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", "Cuan-Platform/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute validation request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrInvalidAPIKey
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var apiErr ResendAPIError
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != "" {
		return &apiErr
	}
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
}

// SendEmail dispatches an email through Resend API.
func (c *HTTPClient) SendEmail(ctx context.Context, req EmailRequest) (*EmailResponse, error) {
	if c.apiKey == "" {
		return nil, ErrEmptyAPIKey
	}
	if strings.TrimSpace(req.From) == "" {
		return nil, ErrEmptySender
	}
	if len(req.To) == 0 {
		return nil, ErrEmptyRecipients
	}
	if strings.TrimSpace(req.Subject) == "" {
		return nil, ErrEmptySubject
	}
	if strings.TrimSpace(req.HTML) == "" && strings.TrimSpace(req.Text) == "" {
		return nil, ErrEmptyContent
	}

	payloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal email payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/emails", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("create send email request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "Cuan-Platform/1.0")
	if req.IdempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("dispatch send email request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 16384))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var apiErr ResendAPIError
		if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Message != "" {
			return nil, &apiErr
		}
		return nil, fmt.Errorf("resend send failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var emailResp EmailResponse
	if err := json.Unmarshal(bodyBytes, &emailResp); err != nil {
		return nil, fmt.Errorf("decode email response: %w", err)
	}
	return &emailResp, nil
}

// ChatMessageItem represents a message snippet in an email transcript.
type ChatMessageItem struct {
	SenderName string
	SenderRole string // "customer", "operator", "visitor"
	Content    string
	SentAt     time.Time
}

// ChatIdleTemplateParams contains data required to generate an idle transcript email.
type ChatIdleTemplateParams struct {
	StoreName       string
	CustomerName    string
	CustomerEmail   string
	RoomID          string
	ConversationURL string
	IdleMinutes     int
	Messages        []ChatMessageItem
}

// BuildChatIdleTranscriptHTML generates an email HTML notification with responsive design.
func BuildChatIdleTranscriptHTML(params ChatIdleTemplateParams) string {
	brand := params.StoreName
	if brand == "" {
		brand = "Cuan"
	}
	customer := params.CustomerName
	if customer == "" {
		customer = "Pengunjung"
	}
	var sb strings.Builder
	sb.WriteString(`<!doctype html><html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Pesan belum dibaca</title></head><body style="margin:0;background:#f7f7f5;color:#292a2d;font-family:Geist,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif"><table role="presentation" style="width:100%;border-collapse:collapse"><tr><td style="padding:40px 24px"><table role="presentation" style="width:100%;max-width:600px;margin:0 auto;border-collapse:collapse"><tr><td style="font-size:16px;font-weight:600;padding-bottom:32px">` + html.EscapeString(brand) + `</td></tr><tr><td><h1 style="font-size:24px;line-height:1.3;font-weight:600;margin:0 0 12px">Ada pesan belum dibaca</h1><p style="font-size:15px;line-height:1.6;margin:0 0 28px;color:#62646a">` + html.EscapeString(customer) + ` mengirim pesan yang belum dibuka selama ` + fmt.Sprint(params.IdleMinutes) + ` menit.</p>`)
	if params.CustomerEmail != "" {
		sb.WriteString(`<p style="font-size:13px;color:#62646a;margin:0 0 24px">` + html.EscapeString(params.CustomerEmail) + `</p>`)
	}
	for _, msg := range params.Messages {
		author := msg.SenderName
		if author == "" {
			author = customer
		}
		sb.WriteString(`<div style="padding:16px 0;border-top:1px solid #e3e3df"><p style="font-size:13px;font-weight:600;margin:0 0 8px">` + html.EscapeString(author) + `</p><div style="font-size:15px;line-height:1.6;white-space:pre-wrap;overflow-wrap:anywhere">` + html.EscapeString(msg.Content) + `</div></div>`)
	}
	sb.WriteString(`<p style="margin:28px 0 32px"><a href="` + html.EscapeString(params.ConversationURL) + `" style="display:inline-block;padding:12px 20px;background:#e5e8ec;color:#303b4a;border-radius:8px;text-decoration:none;font-size:14px;font-weight:600;line-height:20px">Buka percakapan</a></p><p style="font-size:12px;line-height:1.6;color:#707278;margin:0">Notifikasi otomatis dari Cuan untuk percakapan yang belum dibaca.</p></td></tr></table></td></tr></table></body></html>`)
	return sb.String()
}

// BuildChatIdleTranscriptText generates a plain-text version for email clients without HTML.
func BuildChatIdleTranscriptText(params ChatIdleTemplateParams) string {
	store := params.StoreName
	if store == "" {
		store = "Cuan"
	}
	customer := params.CustomerName
	if customer == "" {
		customer = "Pengunjung"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s · Notifikasi Live Chat]\n", store))
	sb.WriteString(fmt.Sprintf("Pesan baru dari %s belum terbaca (> %d menit idle).\n\n", customer, params.IdleMinutes))
	if params.CustomerEmail != "" {
		sb.WriteString(fmt.Sprintf("Email Pelanggan: %s\n", params.CustomerEmail))
	}
	sb.WriteString("----------------------------------------\n")
	for _, msg := range params.Messages {
		author := msg.SenderName
		if author == "" {
			author = customer
		}
		sb.WriteString(fmt.Sprintf("%s (%s):\n%s\n\n", author, msg.SentAt.Format("15:04 WIB"), msg.Content))
	}
	sb.WriteString("----------------------------------------\n")
	sb.WriteString(fmt.Sprintf("Buka percakapan di Cuan: %s\n", params.ConversationURL))
	return sb.String()
}
