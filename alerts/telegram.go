package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Service struct {
	BusinessBotToken  string
	BusinessChatID    string
	DeveloperBotToken string
	DeveloperChatID   string
	httpClient        *http.Client
}

func New(businessBot, businessChat, devBot, devChat string) *Service {
	return &Service{
		BusinessBotToken:  businessBot,
		BusinessChatID:    businessChat,
		DeveloperBotToken: devBot,
		DeveloperChatID:   devChat,
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
		"chat_id":    chatID,
		"text":       message,
		"parse_mode": "HTML",
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
