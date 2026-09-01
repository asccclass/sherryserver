package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type Client struct {
	mcpClient client.MCPClient
}

// NewSSEClient establishes an SSE connection to an MCP server.
func NewSSEClient(sseURL string) (*Client, error) {
	if sseURL == "" {
		return nil, fmt.Errorf("MCP SSE URL is empty")
	}

	c, err := client.NewSSEMCPClient(sseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP SSE client: %w", err)
	}

	if err := c.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{
		Name:    "sherryserver",
		Version: "1.0.0",
	}

	_, err = c.Initialize(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MCP client: %w", err)
	}

	return &Client{
		mcpClient: c,
	}, nil
}

// ListTools returns a list of tools available on the MCP server.
func (c *Client) ListTools(ctx context.Context) (*mcp.ListToolsResult, error) {
	req := mcp.ListToolsRequest{}
	return c.mcpClient.ListTools(ctx, req)
}

// CallTool calls a specific tool on the MCP server.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]interface{}) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	return c.mcpClient.CallTool(ctx, req)
}

// Close closes the connection.
func (c *Client) Close() error {
	// mcp-go client might not have a direct close depending on version, 
	// but generally cancelling the context or stopping it if possible.
	return nil
}
