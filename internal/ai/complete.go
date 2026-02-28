package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
)

var ErrBilling = errors.New("I need money: https://rcy.sh/fundannie")

var calgaryTool = openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
	Name:        "calgary",
	Description: openai.String("answer what is happening in calgary"),
	Parameters: openai.FunctionParameters{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	},
})

var echoTool = openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
	Name:        "echo",
	Description: openai.String("echoes back the provided message"),
	Parameters: openai.FunctionParameters{
		"type": "object",
		"properties": map[string]any{
			"message": map[string]any{
				"type":        "string",
				"description": "the message to echo",
			},
		},
		"required": []string{"message"},
	},
})

func dispatchTool(name string, arguments string) (string, error) {
	switch name {
	case "calgary":
		return "calgary is not doing well, in fact its really bad, but the sky is still blue", nil
	case "echo":
		var args struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("echo: invalid arguments: %w", err)
		}
		return args.Message, nil
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func Complete(ctx context.Context, model string, systemPrompt string, userPrompt string) (string, error) {
	client := openai.NewClient()

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	for {
		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:             model,
			Messages:          messages,
			Tools:             []openai.ChatCompletionToolUnionParam{calgaryTool, echoTool},
			ParallelToolCalls: openai.Bool(false),
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

		choice := resp.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		messages = append(messages, choice.Message.ToParam())

		for _, tc := range choice.Message.ToolCalls {
			fmt.Println("toolcall: ", tc.Function.Name, tc.Function.Arguments)
			result, err := dispatchTool(tc.Function.Name, tc.Function.Arguments)
			if err != nil {
				return "", err
			}
			messages = append(messages, openai.ToolMessage(result, tc.ID))
		}
	}
}
