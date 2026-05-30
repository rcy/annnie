package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type deepSeekMessage struct {
	ReasoningContent string `json:"reasoning_content"`
}

type DeepSeekBalanceInfo struct {
	Currency        string `json:"currency"`
	TotalBalance    string `json:"total_balance"`
	GrantedBalance  string `json:"granted_balance"`
	ToppedUpBalance string `json:"topped_up_balance"`
}

type DeepSeekBalance struct {
	IsAvailable  bool                  `json:"is_available"`
	BalanceInfos []DeepSeekBalanceInfo `json:"balance_infos"`
}

func GetDeepSeekBalance(ctx context.Context) (*DeepSeekBalance, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.deepseek.com/user/balance", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("DEEPSEEK_API_KEY"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%s: %s", resp.Status, body)
	}

	var balance DeepSeekBalance
	if err := json.NewDecoder(resp.Body).Decode(&balance); err != nil {
		return nil, err
	}
	return &balance, nil
}

var deepSeekTools = []openai.ChatCompletionToolUnionParam{
	openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        "get_current_time",
		Description: openai.String("Returns the current date and time."),
		Parameters: openai.FunctionParameters{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		},
	}),
}

func handleDeepSeekTool(name string) (string, error) {
	switch name {
	case "get_current_time":
		return time.Now().Format(time.RFC1123), nil
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

func CompleteDeepSeek(ctx context.Context, systemPrompt string, userPrompt string) (string, error) {
	diagFn := diagFuncFromContext(ctx)
	if diagFn == nil {
		return "", fmt.Errorf("diagFuncFromContext did not return a function")
	}

	client := openai.NewClient(
		option.WithBaseURL("https://api.deepseek.com/v1"),
		option.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
	)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	for {
		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    getModel(ctx),
			Messages: messages,
			Tools:    deepSeekTools,
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
		msg := choice.Message

		var dsMsg deepSeekMessage
		if err := json.Unmarshal([]byte(msg.RawJSON()), &dsMsg); err == nil && dsMsg.ReasoningContent != "" {
			diagFn("* " + dsMsg.ReasoningContent)
		}

		if choice.FinishReason == "tool_calls" {
			messages = append(messages, msg.ToParam())
			for _, call := range msg.ToolCalls {
				diagFn("> tool: " + call.Function.Name)
				result, err := handleDeepSeekTool(call.Function.Name)
				if err != nil {
					return "", err
				}
				diagFn("< tool: " + result)
				messages = append(messages, openai.ToolMessage(result, call.ID))
			}
			continue
		}

		diagFn("END")

		return msg.Content, nil
	}
}
