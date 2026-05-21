package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
)

func CompleteOpenAI(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	client := openai.NewClient()

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

	return resp.Choices[0].Message.Content, nil
}
