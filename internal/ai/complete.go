package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
)

var ErrBilling = errors.New("I need money: https://rcy.sh/fundannie")

func Complete(ctx context.Context, systemPrompt string, userPrompt string, websearch bool) (string, error) {
	ctx, _ = context.WithTimeout(ctx, 30*time.Second)

	client := openai.NewClient()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	for {
		options := openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4oMini,
			Messages: messages,
		}
		if websearch {
			options.Model = openai.ChatModelGPT4oMiniSearchPreview
			options.WebSearchOptions = openai.ChatCompletionNewParamsWebSearchOptions{
				SearchContextSize: "low",
			}
		}

		resp, err := client.Chat.Completions.New(ctx, options)
		if err != nil {
			if strings.Contains(err.Error(), "billing") {
				return "", ErrBilling
			}
			return "", fmt.Errorf("chat completion failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no completion choices returned")
		}

		choice := resp.Choices[0]

		if len(choice.Message.Annotations) == 0 {
			return choice.Message.Content, nil
		}

		// there were annotations, so result was based on web search
		condensed, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: openai.ChatModelGPT4oMini,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage("Summarize the following into a single sentence, in lower case, with minimal punctuation."),
				openai.UserMessage(choice.Message.Content),
			},
		})
		if err != nil {
			return "", fmt.Errorf("condense failed: %w", err)
		}
		if len(condensed.Choices) == 0 {
			return "", fmt.Errorf("no condense choices returned")
		}
		return "*" + condensed.Choices[0].Message.Content, nil
	}
}
