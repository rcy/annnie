package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	gmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	Name     string
	Server   string
	Help     string
	Input    map[string]any
	Required []string
}

type serverConn struct {
	name    string
	session *gmcp.ClientSession
}

type Client struct {
	mu      sync.Mutex
	tools   map[string]*Tool // flat map: "server:toolname" -> Tool
	servers []*serverConn
}

func NewClient(ctx context.Context, urls ...string) (*Client, error) {
	c := &Client{
		tools: make(map[string]*Tool),
	}

	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, "=", 2)
		var name, url string
		if len(parts) == 2 {
			name = strings.TrimSpace(parts[0])
			url = strings.TrimSpace(parts[1])
		} else {
			url = raw
			h := url
			h = strings.TrimPrefix(h, "https://")
			h = strings.TrimPrefix(h, "http://")
			h = strings.SplitN(h, "/", 2)[0]
			h = strings.SplitN(h, ":", 2)[0]
			name = strings.SplitN(h, ".", 2)[0]
		}

		sc, list, err := connectServer(ctx, name, url)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		c.servers = append(c.servers, sc)

		for _, t := range list {
			key := name + ":" + t.Name
			tool := &Tool{
				Name:   key,
				Server: name,
				Help:   t.Description,
			}
			if schema, ok := t.InputSchema.(map[string]any); ok {
				if props, ok := schema["properties"].(map[string]any); ok {
					tool.Input = props
				}
				if req, ok := schema["required"].([]any); ok {
					for _, r := range req {
						if s, ok := r.(string); ok {
							tool.Required = append(tool.Required, s)
						}
					}
				}
			}
			c.tools[key] = tool
		}
	}

	return c, nil
}

func connectServer(ctx context.Context, name, url string) (*serverConn, []*gmcp.Tool, error) {
	client := gmcp.NewClient(&gmcp.Implementation{
		Name:    "annie-mcp",
		Version: "0.1.0",
	}, nil)

	transport := &gmcp.SSEClientTransport{Endpoint: url}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		_ = session.Close()
		return nil, nil, fmt.Errorf("list tools: %w", err)
	}

	return &serverConn{name: name, session: session}, result.Tools, nil
}

func (c *Client) ListTools() []*Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	tools := make([]*Tool, 0, len(c.tools))
	for _, t := range c.tools {
		tools = append(tools, t)
	}
	return tools
}

func (c *Client) FindTool(name string) *Tool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tools[name]
}

func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	c.mu.Lock()
	tool, ok := c.tools[name]
	c.mu.Unlock()

	if !ok {
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	for _, sc := range c.servers {
		if sc.name == tool.Server {
			result, err := sc.session.CallTool(ctx, &gmcp.CallToolParams{
				Name:      strings.TrimPrefix(name, sc.name+":"),
				Arguments: args,
			})
			if err != nil {
				return "", err
			}
			if result.IsError {
				msg := "error"
				for _, content := range result.Content {
					if tc, ok := content.(*gmcp.TextContent); ok {
						msg = tc.Text
					}
				}
				return "", fmt.Errorf("%s", msg)
			}
			var out strings.Builder
			for _, content := range result.Content {
				if tc, ok := content.(*gmcp.TextContent); ok {
					if out.Len() > 0 {
						out.WriteString("\n")
					}
					out.WriteString(tc.Text)
				}
			}
			return out.String(), nil
		}
	}
	return "", fmt.Errorf("server not found for tool: %s", name)
}

// FromEnv reads MCP endpoints from the ANNIE_MCP_ENDPOINTS env var.
// Format: comma-separated URLs, or name=url pairs.
func FromEnv(ctx context.Context) (*Client, error) {
	raw := os.Getenv("ANNIE_MCP_ENDPOINTS")
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	return NewClient(ctx, parts...)
}
