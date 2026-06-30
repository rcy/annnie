package ai

import (
	"context"
	"goirc/internal/mcp"
	"strings"
	"sync"
)

type mcpClientKey struct{}

var (
	globalMCPClient     *mcp.Client
	mcpNameMu           sync.Mutex
	mcpNameToRealName   map[string]string // sanitized -> real name ("server_toolname" -> "server:toolname")
)

// SetMCPClient sets the global MCP client for tool calls.
func SetMCPClient(c *mcp.Client) {
	globalMCPClient = c
}

// WithMCPClient stores an MCP client in the context, overriding the global one.
func WithMCPClient(ctx context.Context, c *mcp.Client) context.Context {
	return context.WithValue(ctx, mcpClientKey{}, c)
}

func resolveMCPClient(ctx context.Context) *mcp.Client {
	if c, ok := ctx.Value(mcpClientKey{}).(*mcp.Client); ok {
		return c
	}
	return globalMCPClient
}

// MCPToolName sanitizes an MCP tool name for use with LLM APIs that restrict
// characters (e.g. DeepSeek only allows ^[a-zA-Z0-9_-]+$).
func MCPToolName(realName string) string {
	sanitized := strings.ReplaceAll(realName, ":", "_")
	if sanitized != realName {
		mcpNameMu.Lock()
		if mcpNameToRealName == nil {
			mcpNameToRealName = make(map[string]string)
		}
		mcpNameToRealName[sanitized] = realName
		mcpNameMu.Unlock()
	}
	return sanitized
}

// MCPToolRealName reverses MCPToolName to get the original name.
func MCPToolRealName(sanitized string) string {
	mcpNameMu.Lock()
	real, ok := mcpNameToRealName[sanitized]
	mcpNameMu.Unlock()
	if ok {
		return real
	}
	return sanitized
}
