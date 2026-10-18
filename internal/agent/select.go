// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"strings"
	"time"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// selectJS picks the option of a <select> whose label (or value) is want,
// as a person choosing it from the list does: the change shows on the page
// as input and change events.
const selectJS = `function(want) {
	if (this.tagName !== 'SELECT') return {notSelect: true};
	const norm = s => s.replace(/\s+/g, ' ').trim().toLowerCase();
	const opts = [...this.options];
	const o = opts.find(o => norm(o.label) === norm(want)) || opts.find(o => o.value === want);
	if (!o) return {options: opts.map(o => o.label.trim())};
	if (o.disabled) return {disabled: o.label.trim()};
	this.focus();
	if (!o.selected) {
		o.selected = true;
		this.dispatchEvent(new Event('input', {bubbles: true}));
		this.dispatchEvent(new Event('change', {bubbles: true}));
	}
	return {ok: o.label.trim()};
}`

// SelectOption chooses option in a native <select>. Clicking one opens the
// system's list, which page input cannot reach.
func (b *Browser) SelectOption(ctx context.Context, id, ref, option string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if _, _, err := b.point(ctx, t, ref); err != nil { // scrolled into view, as a person would
		return Tab{}, err
	}
	pause(150*time.Millisecond, 400*time.Millisecond)
	var r struct {
		OK        string   `json:"ok"`
		Options   []string `json:"options"`
		Disabled  string   `json:"disabled"`
		NotSelect bool     `json:"notSelect"`
	}
	if err := b.callOn(ctx, t, ref, selectJS, []any{option}, &r); err != nil {
		return Tab{}, err
	}
	switch {
	case r.NotSelect:
		return Tab{}, apperr.Newf("agent_not_select", "%s is not a <select>; click it, then click the option in the snapshot", ref)
	case r.Disabled != "":
		return Tab{}, apperr.Newf("agent_option_disabled", "the option %q cannot be chosen now", r.Disabled)
	case r.OK == "":
		return Tab{}, apperr.Newf("agent_no_option", "no option %q; the options are: %s", option, strings.Join(r.Options, ", "))
	}
	pause(300*time.Millisecond, 600*time.Millisecond) // let the page react
	return b.describe(ctx, t), nil
}
