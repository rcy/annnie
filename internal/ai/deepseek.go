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
	openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        "get_sports_scores",
		Description: openai.String("Returns scores for a sports league on a given date (YYYYMMDD). Supported leagues: nfl, nba, mlb, nhl, mls."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"league": map[string]any{
					"type":        "string",
					"description": "The sports league, e.g. nfl, nba, mlb, nhl, mls",
					"enum":        []string{"nfl", "nba", "mlb", "nhl", "mls"},
				},
				"date": map[string]any{
					"type":        "string",
					"description": "The date in YYYYMMDD format, e.g. 20260609",
				},
			},
			"required": []string{"league", "date"},
		},
	}),
}

var espnLeagues = map[string]string{
	"nfl": "football/nfl",
	"nba": "basketball/nba",
	"mlb": "baseball/mlb",
	"nhl": "hockey/nhl",
	"mls": "soccer/usa.1",
}

func getSportsScores(ctx context.Context, league string, date string) (string, error) {
	path, ok := espnLeagues[league]
	if !ok {
		return "", fmt.Errorf("unknown league: %s", league)
	}

	url := "https://site.api.espn.com/apis/site/v2/sports/" + path + "/scoreboard"
	if date == "" {
		return "", fmt.Errorf("date missing")
	}
	url += "?dates=" + date

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%s: %s", resp.Status, body)
	}

	var result struct {
		Events []struct {
			Name      string `json:"name"`
			ShortName string `json:"shortName"`
			Status    struct {
				Type struct {
					Description string `json:"description"`
					ShortDetail string `json:"shortDetail"`
					Completed   bool   `json:"completed"`
				} `json:"type"`
				DisplayClock string `json:"displayClock"`
				Period       int    `json:"period"`
			} `json:"status"`
			Competitions []struct {
				Competitors []struct {
					HomeAway string `json:"homeAway"`
					Score    string `json:"score"`
					Team     struct {
						Abbreviation string `json:"abbreviation"`
					} `json:"team"`
				} `json:"competitors"`
			} `json:"competitions"`
		} `json:"events"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Events) == 0 {
		return "No games found.", nil
	}

	var lines []string
	for _, event := range result.Events {
		status := event.Status.Type.Description
		if event.Status.Type.Completed {
			status = "Final"
		} else if event.Status.Period > 0 && event.Status.DisplayClock != "" {
			status = fmt.Sprintf("Q%d %s", event.Status.Period, event.Status.DisplayClock)
		} else if event.Status.Type.ShortDetail != "" {
			status = event.Status.Type.ShortDetail
		}

		score := event.ShortName
		if len(event.Competitions) > 0 {
			comp := event.Competitions[0]
			var away, home string
			for _, c := range comp.Competitors {
				if c.HomeAway == "away" {
					away = fmt.Sprintf("%s %s", c.Team.Abbreviation, c.Score)
				} else {
					home = fmt.Sprintf("%s %s", c.Team.Abbreviation, c.Score)
				}
			}
			if away != "" && home != "" {
				score = fmt.Sprintf("%s @ %s", away, home)
			}
		}

		lines = append(lines, fmt.Sprintf("%s (%s)", score, status))
	}

	return strings.Join(lines, "\n"), nil
}

func handleDeepSeekTool(ctx context.Context, name string, args string) (string, error) {
	switch name {
	case "get_current_time":
		return time.Now().Format(time.RFC1123), nil
	case "get_sports_scores":
		var params struct {
			League string `json:"league"`
			Date   string `json:"date"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid args: %w", err)
		}
		return getSportsScores(ctx, params.League, params.Date)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

type Params struct {
	SystemPrompt string
	UserPrompt   string
	UseTools     bool
}

func CompleteDeepSeek(ctx context.Context, params Params) (string, error) {
	diagFn := diagFuncFromContext(ctx)
	if diagFn == nil {
		return "", fmt.Errorf("diagFuncFromContext did not return a function")
	}

	diagFn("--- " + params.UserPrompt)

	client := openai.NewClient(
		option.WithBaseURL("https://api.deepseek.com/v1"),
		option.WithAPIKey(os.Getenv("DEEPSEEK_API_KEY")),
	)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(params.SystemPrompt),
		openai.UserMessage(params.UserPrompt),
	}

	tools := []openai.ChatCompletionToolUnionParam{}
	if params.UseTools {
		tools = deepSeekTools
	}

	for {
		resp, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    getModel(ctx),
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			if strings.Contains(err.Error(), "billing") {
				return "", ErrBilling
			}
			diagFn("ERR " + err.Error())
			return "", fmt.Errorf("chat completion failed: %w", err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("no completion choices returned")
		}

		choice := resp.Choices[0]
		msg := choice.Message

		var dsMsg deepSeekMessage
		if err := json.Unmarshal([]byte(msg.RawJSON()), &dsMsg); err == nil && dsMsg.ReasoningContent != "" {
			diagFn("RSN " + dsMsg.ReasoningContent)
		}

		if choice.FinishReason == "tool_calls" {
			messages = append(messages, msg.ToParam())
			for _, call := range msg.ToolCalls {
				result, err := handleDeepSeekTool(ctx, call.Function.Name, call.Function.Arguments)
				if err != nil {
					diagFn("ERR " + err.Error())
					return "", err
				}
				diagFn(fmt.Sprintf("TOOL %s(%v) -> %s", call.Function.Name, call.Function.Arguments, result))
				messages = append(messages, openai.ToolMessage(result, call.ID))
			}
			continue
		}

		diagFn("OUT " + msg.Content)

		return msg.Content, nil
	}
}
