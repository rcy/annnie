package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func CompleteDeepSeek(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	client := openai.NewClient(
		option.WithBaseURL("https://api.deepseek.com/v1"),
		option.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
	)

	resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: getModel(ctx),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "billing") {
			return "", ErrBilling
		}
		return "", fmt.Errorf("chat completion failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	content := stripThinking(resp.Choices[0].Message.Content)

	return content, nil
}
