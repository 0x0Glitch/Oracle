package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	BusinessBotToken  string
	BusinessChatID    string
	DeveloperBotToken string
	DeveloperChatID   string
	SlackWebhookURL   string
	httpClient        *http.Client
}

func New(businessBot, businessChat, devBot, devChat, slackWebhook string) *Service {
	return &Service{
		BusinessBotToken:  businessBot,
		BusinessChatID:    businessChat,
		DeveloperBotToken: devBot,
		DeveloperChatID:   devChat,
		SlackWebhookURL:   slackWebhook,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Service) SendBusinessAlert(ctx context.Context, message string) error {
	if s.BusinessBotToken == "" || s.BusinessChatID == "" {
		log.Printf("[alerts] business alerts not configured")
		return nil
	}
	return s.sendTelegram(ctx, s.BusinessBotToken, s.BusinessChatID, message)
}

func (s *Service) SendDeveloperAlert(ctx context.Context, message string) error {
	if s.DeveloperBotToken == "" || s.DeveloperChatID == "" {
		log.Printf("[alerts] developer alerts not configured")
		return nil
	}
	return s.sendTelegram(ctx, s.DeveloperBotToken, s.DeveloperChatID, message)
}

func (s *Service) sendTelegram(ctx context.Context, botToken, chatID, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10)) // Read first 4KB
		return fmt.Errorf("telegram API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (s *Service) SendSlackAlert(ctx context.Context, message string) error {
	if s.SlackWebhookURL == "" {
		log.Printf("[alerts] slack alerts not configured")
		return nil
	}
	return s.sendSlack(ctx, message)
}

func (s *Service) sendSlack(ctx context.Context, message string) error {
	// Convert HTML tags to Slack mrkdwn format
	slackMessage := convertHTMLToSlack(message)

	payload := map[string]interface{}{
		"text": slackMessage,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.SlackWebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send slack request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("slack webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// convertHTMLToSlack converts HTML formatting to Slack mrkdwn
func convertHTMLToSlack(html string) string {
	result := html
	// Convert <b> to *bold*
	result = strings.ReplaceAll(result, "<b>", "*")
	result = strings.ReplaceAll(result, "</b>", "*")
	// Convert <i> to _italic_
	result = strings.ReplaceAll(result, "<i>", "_")
	result = strings.ReplaceAll(result, "</i>", "_")
	// Convert <code> to `code`
	result = strings.ReplaceAll(result, "<code>", "`")
	result = strings.ReplaceAll(result, "</code>", "`")
	return result
}
