// SPDX-License-Identifier: AGPL-3.0-or-later

package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/D0bby999/anyprofile/internal/apperr"
	"github.com/D0bby999/anyprofile/internal/recipe"
)

// promptSet remembers which recipes are offered as prompts, so one removed
// from the folder is withdrawn.
type promptSet struct {
	mu    sync.Mutex
	names map[string]bool
}

// recipes are the user's recipes and the Pro pack's, the user's winning a
// shared name, sorted by name.
func (s *Server) recipes() ([]recipe.Recipe, []error) {
	rs, errs := recipe.Load(s.Recipes)
	if s.ProRecipes == "" {
		return rs, errs
	}
	pro, perrs := recipe.Load(s.ProRecipes)
	mine := map[string]bool{}
	for _, r := range rs {
		mine[r.Name] = true
	}
	for _, r := range pro {
		if !mine[r.Name] {
			r.Pro = true
			rs = append(rs, r)
		}
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].Name < rs[j].Name })
	return rs, append(errs, perrs...)
}

// syncPrompts offers every recipe in the folder as an MCP prompt; Claude Code
// shows them as /mcp__anyprofile__<name>. It runs on every request, so a file
// dropped in the folder is there on the next one.
func (s *Server) syncPrompts(srv *mcp.Server) {
	rs, _ := s.recipes()
	s.prompts.mu.Lock()
	defer s.prompts.mu.Unlock()
	now := map[string]bool{}
	for _, r := range rs {
		now[r.Name] = true
		args := make([]*mcp.PromptArgument, len(r.Args))
		for i, a := range r.Args {
			args[i] = &mcp.PromptArgument{Name: a.Name, Description: a.Description, Required: a.Required}
		}
		name := r.Name
		srv.AddPrompt(&mcp.Prompt{Name: name, Description: r.Description, Arguments: args},
			func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
				return s.renderPrompt(name, req.Params.Arguments)
			})
	}
	var gone []string
	for n := range s.prompts.names {
		if !now[n] {
			gone = append(gone, n)
		}
	}
	if len(gone) > 0 {
		srv.RemovePrompts(gone...)
	}
	s.prompts.names = now
}

// renderPrompt reads the recipe again, so the prompt is what the file says now.
func (s *Server) renderPrompt(name string, args map[string]string) (*mcp.GetPromptResult, error) {
	rs, _ := s.recipes()
	for _, r := range rs {
		if r.Name != name {
			continue
		}
		body, err := r.Render(args)
		if err != nil {
			return nil, err
		}
		s.log.Info("mcp", "prompt", name)
		return &mcp.GetPromptResult{Description: r.Description, Messages: []*mcp.PromptMessage{
			{Role: "user", Content: &mcp.TextContent{Text: recipeHead(r) + body}},
		}}, nil
	}
	return nil, apperr.Newf("recipe_not_found", "no recipe %q", name)
}

func recipeHead(r recipe.Recipe) string {
	v := ""
	if r.Version != "" {
		v = " v" + r.Version
	}
	return fmt.Sprintf("Recipe %s%s for AnyProfile. Follow it with the anyprofile tools, one step at a time, and stop where it says to ask the user.\n\n", r.Name, v)
}

func recipeList(rs []recipe.Recipe, errs []error) string {
	var sb strings.Builder
	if len(rs) == 0 {
		sb.WriteString("No recipes yet. The user adds them to AnyProfile's recipes folder (Settings → Local API → Open recipes folder).\n")
	}
	for _, r := range rs {
		var args []string
		for _, a := range r.Args {
			n := a.Name
			if a.Required {
				n += "*"
			}
			args = append(args, n)
		}
		pro := ""
		if r.Pro {
			pro = " (Pro)"
		}
		fmt.Fprintf(&sb, "- %s%s: %s", r.Name, pro, r.Description)
		if len(args) > 0 {
			fmt.Fprintf(&sb, " (arguments: %s; * = required)", strings.Join(args, ", "))
		}
		sb.WriteByte('\n')
	}
	for _, e := range errs {
		fmt.Fprintf(&sb, "Not loaded: %v\n", e)
	}
	return sb.String()
}
