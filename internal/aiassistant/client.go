package aiassistant

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

type Analysis struct {
	Category       string  `json:"category"`
	Confidence     float64 `json:"confidence"`
	Recommendation string  `json:"recommendation"`
	Rationale      string  `json:"rationale"`
}

type request struct {
	ChatID    int64  `json:"chat_id"`
	UserID    int64  `json:"user_id"`
	MessageID int64  `json:"message_id"`
	Feature   string `json:"feature"`
	Text      string `json:"text"`
}

type response struct {
	Analysis Analysis `json:"analysis"`
	Mode string `json:"mode"`
	EnforcementEnabled bool `json:"enforcement_enabled"`
}

const maxAIResponseBytes = 64 * 1024

type Client struct {
	endpoint string
	token string
	http *http.Client
}

func NewClient(baseURL, serviceToken string) *Client {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	serviceToken = strings.TrimSpace(serviceToken)
	if baseURL == "" || serviceToken == "" {
		return nil
	}
	return &Client{
		endpoint: baseURL + "/v1/ai/analyze",
		token: serviceToken,
		http: &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *Client) Analyze(ctx context.Context, chatID, userID, messageID int64, feature, text string) (Analysis, error) {
	if c == nil {
		return Analysis{}, fmt.Errorf("AI assistance is not configured")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return Analysis{}, fmt.Errorf("message has no text to analyze")
	}
	if len([]rune(text)) > 4000 {
		text = string([]rune(text)[:4000])
	}
	body, err := json.Marshal(request{
		ChatID: chatID, UserID: userID, MessageID: messageID,
		Feature: feature, Text: text,
	})
	if err != nil {
		return Analysis{}, fmt.Errorf("encode AI request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return Analysis{}, fmt.Errorf("create AI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-AI-Service-Token", c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return Analysis{}, fmt.Errorf("AI service unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Analysis{}, fmt.Errorf("AI service returned HTTP %d", resp.StatusCode)
	}
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBytes+1))
	if err != nil {
		return Analysis{}, fmt.Errorf("read AI response: %w", err)
	}
	if len(responseBody) > maxAIResponseBytes {
		return Analysis{}, fmt.Errorf("AI service response exceeds %d bytes", maxAIResponseBytes)
	}
	var decoded response
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return Analysis{}, fmt.Errorf("decode AI response: %w", err)
	}
	if decoded.EnforcementEnabled || decoded.Mode != "assist" {
		return Analysis{}, fmt.Errorf("AI service returned an unsupported enforcement mode")
	}
	if decoded.Analysis.Recommendation != "allow" && decoded.Analysis.Recommendation != "admin_review" {
		return Analysis{}, fmt.Errorf("AI service returned an invalid recommendation")
	}
	if decoded.Analysis.Confidence < 0 || decoded.Analysis.Confidence > 1 {
		return Analysis{}, fmt.Errorf("AI service returned invalid confidence")
	}
	return decoded.Analysis, nil
}
