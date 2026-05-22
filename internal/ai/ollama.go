package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []OllamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type OllamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Model   string        `json:"model"`
	Message OllamaMessage `json:"message"`
	Done    bool          `json:"done"`
}

func CompleteOllama(ctx context.Context, systemPrompt string, prompt string) (string, error) {
	result, err := completeOllamaResponse(ctx, systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("CompleteOllama: %w", err)
	}

	content := stripThinking(result.Message.Content)

	return content, err
}

func stripThinking(s string) string {
	_, after, found := strings.Cut(s, "</think>")
	if found {
		return strings.TrimSpace(after)
	}
	return s
}

func completeOllamaResponse(ctx context.Context, systemPrompt string, prompt string) (*ollamaResponse, error) {
	body, err := json.Marshal(ollamaRequest{
		Model:  getModel(ctx),
		Stream: false,
		Messages: []OllamaMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://ollama.com/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+os.Getenv("OLLAMA_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("Status: %s (error reading body: %w)", resp.Status, err)
		}
		return nil, fmt.Errorf("%s: %s", resp.Status, body)
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
