// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// ensureClickable refuses to click through an overlay. It checks the point in
// the top frame; an iframe has its own document, so refs inside one skip the
// check rather than risk a false positive.
func (b *Browser) ensureClickable(ctx context.Context, t *tab, ref string) error {
	node, f, err := b.resolve(t, ref)
	if err != nil {
		return err
	}
	if f != nil {
		return nil
	}
	x, y, err := b.point(ctx, t, ref)
	if err != nil {
		return err
	}
	var obj struct {
		Object struct {
			ObjectID string `json:"objectId"`
		} `json:"object"`
	}
	if err := b.call(ctx, t.session, "DOM.resolveNode", map[string]any{"backendNodeId": node}, &obj); err != nil {
		return errGone(ref)
	}
	if obj.Object.ObjectID == "" {
		return nil
	}
	defer b.call(context.Background(), t.session, "Runtime.releaseObject", map[string]any{"objectId": obj.Object.ObjectID}, nil)

	hit := func() (bool, string) {
		var r struct {
			Result struct {
				Value string `json:"value"`
			} `json:"result"`
		}
		fn := `function(x, y) {
			const el = document.elementFromPoint(x, y);
			if (!el) return 'nothing';
			const ok = el === this || this.contains(el);
			return ok ? 'ok' : (el.tagName.toLowerCase() + (el.id ? '#' + el.id : ''));
		}`
		err := b.call(ctx, t.session, "Runtime.callFunctionOn", map[string]any{
			"objectId": obj.Object.ObjectID, "functionDeclaration": fn, "returnByValue": true,
			"arguments": []map[string]any{{"value": x}, {"value": y}},
		}, &r)
		if err != nil {
			return true, ""
		}
		return r.Result.Value == "ok", r.Result.Value
	}
	if ok, desc := hit(); !ok {
		// An overlay may be a menu that closes on its own; one short retry.
		time.Sleep(350 * time.Millisecond)
		if ok, desc = hit(); !ok {
			return apperr.Newf("agent_covered", "the click point is covered by %s", desc)
		}
	}
	return nil
}
