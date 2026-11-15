// Package telegram mengirim pesan ke satu chat admin lewat Bot API Telegram.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client mengirim pesan ke chat admin.
type Client struct {
	token   string
	chatID  string
	baseURL string
	http    *http.Client
}

// NewClient membuat Client. Bila token atau chatID kosong, client nonaktif
// (Enabled() == false) dan pengiriman dilewati.
func NewClient(token, chatID string) *Client {
	return &Client{
		token:   strings.TrimSpace(token),
		chatID:  strings.TrimSpace(chatID),
		baseURL: "https://api.telegram.org",
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Enabled melaporkan apakah token dan chatID tersedia.
func (c *Client) Enabled() bool {
	return c != nil && c.token != "" && c.chatID != ""
}

// SendMessage mengirim teks ke chat admin.
func (c *Client) SendMessage(ctx context.Context, text string) error {
	if !c.Enabled() {
		return fmt.Errorf("telegram: token/chat id belum dikonfigurasi")
	}
	body, err := json.Marshal(map[string]any{
		"chat_id":                  c.chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	})
	if err != nil {
		return fmt.Errorf("telegram: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.baseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: kirim: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram: status %d: %s", resp.StatusCode, strings.TrimSpace(string(preview)))
	}
	return nil
}
