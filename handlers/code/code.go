package code

import (
	"fmt"
	"goirc/internal/ai"
	"goirc/internal/responder"
	"log"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/shared"
)

const systemPrompt = `<instructions>
* You are annnie's code assistant.
* You write and maintain Lua functions stored in a persistent script.
* The script is loaded into the Lua runtime on every restart.
* Available Lua built-ins: print(...), http.get(url), http.json(url).
* No other libraries are available. All code runs in a sandbox.
* Your process:
  1. Call view_lua_script to see the current script.
  2. Write or modify functions. Test with execute_lua.
  3. Call save_lua_script with the ENTIRE script (all functions, old and new).
  4. Report what you did in a short message.
* Never remove existing functions unless explicitly asked.
* Write concise, working Lua code. No placeholder functions.
</instructions>`

var Tools = []openai.ChatCompletionToolUnionParam{
	openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        "execute_lua",
		Description: openai.String("Execute Lua code in a persistent sandbox and return the output. Use this to test functions before saving. Available: print(...), http.get(url), http.json(url)."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "The Lua code to execute.",
				},
			},
			"required": []string{"code"},
		},
	}),
	openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        "save_lua_script",
		Description: openai.String("Persist the Lua script that loads on every restart. Always include ALL existing functions plus new additions. Never remove functions unless asked. Test your code with execute_lua first."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"code": map[string]any{
					"type":        "string",
					"description": "The complete Lua code to persist.",
				},
			},
			"required": []string{"code"},
		},
	}),
	openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
		Name:        "view_lua_script",
		Description: openai.String("View the currently persisted Lua script. Always call this before editing."),
		Parameters: openai.FunctionParameters{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		},
	}),
}

func Handle(params responder.Responder) error {
	if len(params.Matches()) < 2 {
		return nil
	}
	msg := strings.TrimSpace(params.Matches()[1])

	ctx := ai.WithDiagFunc(params.Context(), func(s string) { log.Print(s) })

	response, err := ai.Complete(ctx, ai.Params{
		SystemPrompt: systemPrompt,
		UserPrompt:   fmt.Sprintf("<%s> %s", params.Nick(), msg),
		UseTools:     true,
		Tools:        Tools,
	})
	if err != nil {
		return err
	}
	params.Privmsgf(params.Target(), "%s: %s", params.Nick(), response)

	return nil
}
