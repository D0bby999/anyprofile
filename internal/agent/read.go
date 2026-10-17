// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const readJS = `function(links) {
  const el = this === window || this === undefined ? document.body : this;
  if (!el) return {text: ''};
  const text = el.innerText ?? el.textContent ?? '';
  if (!links) return {text};
  const as = [...(el.querySelectorAll ? el.querySelectorAll('a[href]') : [])].slice(0, 500);
  return {text, links: as.map(a => [(a.innerText || a.title || '').trim().replace(/\s+/g, ' ').slice(0, 120), a.href])};
}`

// ReadPage returns the visible text of the page, or of ref, from offset on and
// at most MaxText characters; with links, also the links' text and address.
func (b *Browser) ReadPage(ctx context.Context, id, ref string, offset int, links bool) (string, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return "", err
	}
	var v struct {
		Text  string      `json:"text"`
		Links [][2]string `json:"links"`
	}
	if ref == "" {
		if err := b.evalJSON(ctx, t, fmt.Sprintf("(%s).call(undefined, %v)", readJS, links), &v); err != nil {
			return "", err
		}
	} else if err := b.callOn(ctx, t, ref, readJS, []any{links}, &v); err != nil {
		return "", err
	}
	page := b.describe(ctx, t)
	var sb strings.Builder
	fmt.Fprintf(&sb, "Tab %s: %s\nURL: %s\n\n", page.ID, page.Title, page.URL)
	text := []rune(v.Text)
	if offset < 0 || offset > len(text) {
		offset = len(text)
	}
	end := min(len(text), offset+MaxText)
	sb.WriteString(string(text[offset:end]))
	if end < len(text) {
		fmt.Fprintf(&sb, "\n… %d more characters; call read_page with offset=%d.", len(text)-end, end)
	}
	if links && offset == 0 {
		sb.WriteString("\n\nLinks:\n")
		for _, l := range v.Links {
			fmt.Fprintf(&sb, "- %s → %s\n", l[0], l[1])
		}
	}
	return sb.String(), nil
}

// callOn runs fn with this set to the element of ref, in the isolated world.
func (b *Browser) callOn(ctx context.Context, t *tab, ref, fn string, args []any, out any) error {
	node, err := parseRef(ref)
	if err != nil {
		return err
	}
	world, err := b.world(ctx, t)
	if err != nil {
		return err
	}
	var obj struct {
		Object struct {
			ObjectID string `json:"objectId"`
		} `json:"object"`
	}
	if err := b.call(ctx, t.session, "DOM.resolveNode", map[string]any{"backendNodeId": node, "executionContextId": world}, &obj); err != nil {
		return errGone(ref)
	}
	defer b.call(context.Background(), t.session, "Runtime.releaseObject", map[string]any{"objectId": obj.Object.ObjectID}, nil)
	callArgs := make([]map[string]any, len(args))
	for i, a := range args {
		callArgs[i] = map[string]any{"value": a}
	}
	var r evalReply
	if err := b.call(ctx, t.session, "Runtime.callFunctionOn", map[string]any{
		"functionDeclaration": fn, "objectId": obj.Object.ObjectID, "arguments": callArgs, "returnByValue": true,
	}, &r); err != nil {
		return err
	}
	if r.ExceptionDetails != nil {
		return fmt.Errorf("page script: %s %s", r.ExceptionDetails.Text, r.ExceptionDetails.Exception.Description)
	}
	return json.Unmarshal(r.Result.Value, out)
}
