package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"goirc/internal/mcp"
	"goirc/internal/responder"
)

type key struct{}

var client *mcp.Client

// WithContext stores the MCP client in the context.
func WithContext(ctx context.Context, c *mcp.Client) context.Context {
	return context.WithValue(ctx, key{}, c)
}

// FromContext retrieves the MCP client from the context.
func FromContext(ctx context.Context) *mcp.Client {
	c, _ := ctx.Value(key{}).(*mcp.Client)
	return c
}

// SetClient sets the MCP client globally. Called during startup.
func SetClient(c *mcp.Client) {
	client = c
}

func Handle(params responder.Responder) error {
	c := client
	if c == nil {
		c = FromContext(params.Context())
	}
	if c == nil {
		params.Privmsgf(params.Target(), "no MCP endpoints configured")
		return nil
	}

	args := strings.Fields(params.Msg())

	if len(args) == 1 {
		// list tools
		tools := c.ListTools()
		if len(tools) == 0 {
			params.Privmsgf(params.Target(), "no MCP tools available")
			return nil
		}
		sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
		lines := make([]string, len(tools))
		for i, t := range tools {
			desc := t.Help
			if desc == "" {
				desc = "no description"
			}
			lines[i] = fmt.Sprintf("%s: %s", t.Name, desc)
		}
		params.Privmsgf(params.Target(), "MCP tools: %s", strings.Join(lines, " | "))
		return nil
	}

	// !mcp <toolname> [json args]
	toolName := args[1]
	rest := strings.Join(args[2:], " ")

	tool := c.FindTool(toolName)
	if tool == nil {
		params.Privmsgf(params.Target(), "unknown MCP tool: %s", toolName)
		return nil
	}

	var callArgs map[string]any
	if rest != "" {
		// try JSON first
		if err := json.Unmarshal([]byte(rest), &callArgs); err != nil {
			// try key=val pairs
			callArgs = make(map[string]any)
			for _, pair := range strings.Fields(rest) {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					callArgs[kv[0]] = kv[1]
				}
			}
		}
	}

	if callArgs == nil {
		callArgs = map[string]any{}
	}

	result, err := c.CallTool(params.Context(), toolName, callArgs)
	if err != nil {
		return err
	}

	params.Privmsgf(params.Target(), "%s: %s", toolName, result)
	return nil
}
