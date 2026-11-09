// SPDX-License-Identifier: AGPL-3.0-or-later

// mcp-call drives the app's MCP endpoint the way an outside agent does: list
// the tools, or call one with JSON arguments. It exists so the MCP surface can
// be tested without a chat application.
//
//	mcp-call tools
//	mcp-call call list_profiles '{}'
//	mcp-call call navigate '{"url":"https://www.youtube.com/upload"}'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type authTransport struct{ tok string }

func (a authTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+a.tok)
	return http.DefaultTransport.RoundTrip(r)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mcp-call tools | call <tool> [json]")
		os.Exit(2)
	}
	tok := os.Getenv("ANYPROFILE_TOKEN")
	if tok == "" {
		fmt.Fprintln(os.Stderr, "ANYPROFILE_TOKEN is empty")
		os.Exit(2)
	}
	endpoint := os.Getenv("ANYPROFILE_MCP")
	if endpoint == "" {
		endpoint = "http://127.0.0.1:51525/mcp"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c := mcp.NewClient(&mcp.Implementation{Name: "mcp-call", Version: "1"}, nil)
	sess, err := c.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: endpoint,
		HTTPClient: &http.Client{Transport: authTransport{tok}}, DisableStandaloneSSE: true}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "connect:", err)
		os.Exit(1)
	}
	defer sess.Close()
	switch os.Args[1] {
	case "tools":
		res, err := sess.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			fmt.Fprintln(os.Stderr, "list tools:", err)
			os.Exit(1)
		}
		for _, t := range res.Tools {
			fmt.Printf("%s\t%s\n", t.Name, firstLine(t.Description))
		}
	case "call":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "call needs a tool name")
			os.Exit(2)
		}
		args := map[string]any{}
		if len(os.Args) > 3 {
			if err := json.Unmarshal([]byte(os.Args[3]), &args); err != nil {
				fmt.Fprintln(os.Stderr, "arguments:", err)
				os.Exit(2)
			}
		}
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: os.Args[2], Arguments: args})
		if err != nil {
			fmt.Fprintln(os.Stderr, "call:", err)
			os.Exit(1)
		}
		for _, content := range res.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				fmt.Println(text.Text)
			}
			if img, ok := content.(*mcp.ImageContent); ok {
				_ = os.WriteFile("screenshot.png", img.Data, 0o644)
				fmt.Printf("Saved image (%d bytes) to screenshot.png\n", len(img.Data))
			}
		}
		if res.IsError {
			os.Exit(1)
		}
	case "batch":
		for i := 2; i < len(os.Args); i += 2 {
			tool := os.Args[i]
			args := map[string]any{}
			if i+1 < len(os.Args) && os.Args[i+1] != "{}" {
				if err := json.Unmarshal([]byte(os.Args[i+1]), &args); err != nil {
					fmt.Fprintln(os.Stderr, "arguments for", tool, err)
					os.Exit(2)
				}
			}
			fmt.Printf("=== %s ===\n", tool)
			res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
			if err != nil {
				fmt.Fprintln(os.Stderr, "call error:", err)
				os.Exit(1)
			}
			for _, content := range res.Content {
				if text, ok := content.(*mcp.TextContent); ok {
					fmt.Println(text.Text)
				}
				if img, ok := content.(*mcp.ImageContent); ok {
					_ = os.WriteFile("screenshot.png", img.Data, 0o644)
					fmt.Printf("Saved image (%d bytes) to screenshot.png\n", len(img.Data))
				}
			}
			if res.IsError {
				os.Exit(1)
			}
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown command", os.Args[1])
		os.Exit(2)
	}
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
