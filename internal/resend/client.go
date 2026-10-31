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
	ErrEmptyAPIKey      = errors.New("resend: API key cannot be empty")
	ErrInvalidAPIKey    = errors.New("resend: invalid API key or unauthorized")
	ErrEmptySender      = errors.New("resend: sender (from) email cannot be empty")
	ErrEmptyRecipients  = errors.New("resend: recipient (to) email list cannot be empty")
	ErrEmptySubject     = errors.New("resend: email subject cannot be empty")
	ErrEmptyContent     = errors.New("resend: email html or text content cannot be empty")
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
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
	ReplyTo string   `json:"reply_to,omitempty"`
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
	StoreName     string
	CustomerName  string
	CustomerEmail string
	RoomID        string
	ConversationURL string
	IdleMinutes   int
	Messages      []ChatMessageItem
}

// BuildChatIdleTranscriptHTML generates an email HTML notification with responsive design.
func BuildChatIdleTranscriptHTML(params ChatIdleTemplateParams) string {
	store := params.StoreName
	if store == "" {
		store = "Toko Cuan"
	}
	customer := params.CustomerName
	if customer == "" {
		customer = "Pelanggan"
	}
	convoURL := params.ConversationURL
	if convoURL == "" {
		convoURL = "/inbox/" + params.RoomID
	}

	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Pesan Baru Belum Terbaca - ` + html.EscapeString(store) + `</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; color: #1e293b; margin: 0; padding: 24px; line-height: 1.5; }
    .container { max-width: 580px; margin: 0 auto; background: #ffffff; border-radius: 12px; border: 1px solid #e2e8f0; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05); }
    .header { background: #0f172a; padding: 24px; text-align: left; }
    .header h1 { margin: 0; color: #ffffff; font-size: 18px; font-weight: 700; letter-spacing: -0.01em; }
    .header p { margin: 4px 0 0; color: #94a3b8; font-size: 13px; }
    .content { padding: 24px; }
    .badge { display: inline-block; padding: 4px 10px; background: #fef3c7; color: #92400e; border-radius: 9999px; font-size: 12px; font-weight: 600; margin-bottom: 16px; }
    .lead { font-size: 15px; margin: 0 0 16px; color: #334155; }
    .message-box { background: #f1f5f9; border-left: 4px solid #0284c7; border-radius: 6px; padding: 14px 16px; margin-bottom: 20px; }
    .msg-header { font-size: 12px; color: #64748b; margin-bottom: 6px; display: flex; justify-content: space-between; }
    .msg-author { font-weight: 600; color: #0f172a; }
    .msg-body { font-size: 14px; color: #1e293b; white-space: pre-wrap; word-break: break-word; }
    .cta-container { text-align: center; margin: 28px 0 16px; }
    .btn { display: inline-block; background: #0284c7; color: #ffffff !important; padding: 12px 28px; font-size: 14px; font-weight: 600; text-decoration: none; border-radius: 8px; box-shadow: 0 2px 4px rgba(2,132,199,0.25); }
    .footer { padding: 16px 24px; background: #f8fafc; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>` + html.EscapeString(store) + ` · Live Chat Cuan</h1>
      <p>Notifikasi Pesan Belum Terbaca (Chat Idle)</p>
    </div>
    <div class="content">
      <div class="badge">🔔 Menunggu Respon &gt; ` + fmt.Sprintf("%d", params.IdleMinutes) + ` menit</div>
      <p class="lead">Halo, ada pesan baru dari <strong>` + html.EscapeString(customer) + `</strong>`)

	if params.CustomerEmail != "" {
		sb.WriteString(` (<a href="mailto:` + html.EscapeString(params.CustomerEmail) + `" style="color:#0284c7;text-decoration:none;">` + html.EscapeString(params.CustomerEmail) + `</a>)`)
	}
	sb.WriteString(` yang belum sempat Anda buka di Live Chat.</p>`)

	for _, msg := range params.Messages {
		author := msg.SenderName
		if author == "" {
			author = customer
		}
		timeStr := ""
		if !msg.SentAt.IsZero() {
			timeStr = msg.SentAt.Format("15:04 WIB")
		}
		sb.WriteString(`
      <div class="message-box">
        <div class="msg-header">
          <span class="msg-author">` + html.EscapeString(author) + `</span>
          <span>` + html.EscapeString(timeStr) + `</span>
        </div>
        <div class="msg-body">` + html.EscapeString(msg.Content) + `</div>
      </div>`)
	}

	sb.WriteString(`
      <div class="cta-container">
        <a href="` + html.EscapeString(convoURL) + `" class="btn">Balas Percakapan di Cuan →</a>
      </div>
    </div>
    <div class="footer">
      Email ini dikirim otomatis oleh integrasi Resend Cuan karena percakapan tidak aktif selama lebih dari ` + fmt.Sprintf("%d", params.IdleMinutes) + ` menit.<br>
      Toko Online Cuan · Mengotomasi Penjualan &amp; Layanan Pelanggan
    </div>
  </div>
</body>
</html>`)

	return sb.String()
}

// BuildChatIdleTranscriptText generates a plain-text version for email clients without HTML.
func BuildChatIdleTranscriptText(params ChatIdleTemplateParams) string {
	store := params.StoreName
	if store == "" {
		store = "Toko Cuan"
	}
	customer := params.CustomerName
	if customer == "" {
		customer = "Pelanggan"
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
