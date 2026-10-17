// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"math/rand/v2"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/D0bby999/anyprofile/internal/apperr"
)

// pause waits a random time between lo and hi, like a person between moves.
// Tests set it to nothing.
var pause = func(lo, hi time.Duration) {
	time.Sleep(lo + time.Duration(rand.Int64N(int64(hi-lo)+1)))
}

// point scrolls the element into view and returns its centre, in CSS pixels
// of the viewport, which is what Input.dispatchMouseEvent takes.
func (b *Browser) point(ctx context.Context, t *tab, ref string) (float64, float64, error) {
	node, err := parseRef(ref)
	if err != nil {
		return 0, 0, err
	}
	if err := b.call(ctx, t.session, "DOM.scrollIntoViewIfNeeded", map[string]any{"backendNodeId": node}, nil); err != nil {
		return 0, 0, errGone(ref)
	}
	var r struct {
		Quads [][]float64 `json:"quads"`
	}
	if err := b.call(ctx, t.session, "DOM.getContentQuads", map[string]any{"backendNodeId": node}, &r); err != nil {
		return 0, 0, errGone(ref)
	}
	for _, q := range r.Quads {
		if len(q) != 8 {
			continue
		}
		minX, maxX, minY, maxY := q[0], q[0], q[1], q[1]
		for i := 0; i < 8; i += 2 {
			minX, maxX = min(minX, q[i]), max(maxX, q[i])
			minY, maxY = min(minY, q[i+1]), max(maxY, q[i+1])
		}
		if maxX-minX >= 1 && maxY-minY >= 1 {
			return (minX + maxX) / 2, (minY + maxY) / 2, nil
		}
	}
	return 0, 0, apperr.Newf("agent_not_visible", "%s has no size on screen; it may be hidden", ref)
}

func (b *Browser) mouse(ctx context.Context, t *tab, typ string, x, y float64, extra map[string]any) error {
	p := map[string]any{"type": typ, "x": x, "y": y}
	for k, v := range extra {
		p[k] = v
	}
	return b.call(ctx, t.session, "Input.dispatchMouseEvent", p, nil)
}

// Click moves the mouse to the element in a few steps and clicks it.
func (b *Browser) Click(ctx context.Context, id, ref string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if err := b.clickRef(ctx, t, ref); err != nil {
		return Tab{}, err
	}
	pause(300*time.Millisecond, 600*time.Millisecond) // let the page react
	return b.describe(ctx, t), nil
}

func (b *Browser) clickRef(ctx context.Context, t *tab, ref string) error {
	x, y, err := b.point(ctx, t, ref)
	if err != nil {
		return err
	}
	// Aim near the centre, not at it to the pixel.
	x += rand.Float64()*4 - 2
	y += rand.Float64()*4 - 2
	b.mu.Lock()
	fx, fy := t.mouseX, t.mouseY
	b.mu.Unlock()
	const steps = 6
	for i := 1; i <= steps; i++ {
		f := float64(i) / steps
		if err := b.mouse(ctx, t, "mouseMoved", fx+(x-fx)*f, fy+(y-fy)*f, nil); err != nil {
			return err
		}
		pause(8*time.Millisecond, 20*time.Millisecond)
	}
	b.mu.Lock()
	t.mouseX, t.mouseY = x, y
	b.mu.Unlock()
	press := map[string]any{"button": "left", "buttons": 1, "clickCount": 1}
	if err := b.mouse(ctx, t, "mousePressed", x, y, press); err != nil {
		return err
	}
	pause(50*time.Millisecond, 120*time.Millisecond)
	return b.mouse(ctx, t, "mouseReleased", x, y, map[string]any{"button": "left", "buttons": 0, "clickCount": 1})
}

// key is one key for Input.dispatchKeyEvent.
type key struct {
	key, code, text string
	vk              int
}

var named = map[string]key{
	"Enter": {"Enter", "Enter", "\r", 13}, "Tab": {"Tab", "Tab", "", 9}, "Escape": {"Escape", "Escape", "", 27},
	"Backspace": {"Backspace", "Backspace", "", 8}, "Delete": {"Delete", "Delete", "", 46},
	"ArrowUp": {"ArrowUp", "ArrowUp", "", 38}, "ArrowDown": {"ArrowDown", "ArrowDown", "", 40},
	"ArrowLeft": {"ArrowLeft", "ArrowLeft", "", 37}, "ArrowRight": {"ArrowRight", "ArrowRight", "", 39},
	"Home": {"Home", "Home", "", 36}, "End": {"End", "End", "", 35},
	"PageUp": {"PageUp", "PageUp", "", 33}, "PageDown": {"PageDown", "PageDown", "", 34},
	"Space": {" ", "Space", " ", 32},
}

// keyFor describes a printable ASCII character; ok is false for others,
// which are typed with Input.insertText.
func keyFor(r rune) (key, bool) {
	s := string(r)
	switch {
	case r >= 'a' && r <= 'z':
		return key{s, "Key" + strings.ToUpper(s), s, int(r - 32)}, true
	case r >= 'A' && r <= 'Z':
		return key{s, "Key" + s, s, int(r)}, true
	case r >= '0' && r <= '9':
		return key{s, "Digit" + s, s, int(r)}, true
	case r == ' ':
		return named["Space"], true
	case r > ' ' && r < 127:
		return key{s, "", s, 0}, true
	}
	return key{}, false
}

const (
	modAlt   = 1
	modCtrl  = 2
	modMeta  = 4
	modShift = 8
)

func (b *Browser) sendKey(ctx context.Context, t *tab, k key, mods int, commands []string) error {
	down := map[string]any{"type": "keyDown", "key": k.key, "code": k.code, "windowsVirtualKeyCode": k.vk, "modifiers": mods}
	if k.text != "" && mods&(modCtrl|modMeta|modAlt) == 0 {
		down["text"], down["unmodifiedText"] = k.text, k.text
	} else {
		down["type"] = "rawKeyDown"
	}
	if len(commands) > 0 {
		down["commands"] = commands
	}
	if err := b.call(ctx, t.session, "Input.dispatchKeyEvent", down, nil); err != nil {
		return err
	}
	pause(20*time.Millisecond, 60*time.Millisecond)
	return b.call(ctx, t.session, "Input.dispatchKeyEvent", map[string]any{
		"type": "keyUp", "key": k.key, "code": k.code, "windowsVirtualKeyCode": k.vk, "modifiers": mods,
	}, nil)
}

// perKey is how much text is typed key by key; the rest is inserted at once,
// so a long text does not take minutes.
const perKey = 300

// Type clicks the element (when ref is given), clears it with replace, types
// text key by key, and presses Enter with submit.
func (b *Browser) Type(ctx context.Context, id, ref, text string, replace, submit bool) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if ref != "" {
		if err := b.clickRef(ctx, t, ref); err != nil {
			return Tab{}, err
		}
		pause(100*time.Millisecond, 250*time.Millisecond)
	}
	if replace {
		mod := modCtrl
		if runtime.GOOS == "darwin" {
			mod = modMeta
		}
		if err := b.sendKey(ctx, t, key{"a", "KeyA", "", 65}, mod, []string{"selectAll"}); err != nil {
			return Tab{}, err
		}
		if err := b.sendKey(ctx, t, named["Backspace"], 0, nil); err != nil {
			return Tab{}, err
		}
	}
	rest := text
	for n := 0; rest != "" && n < perKey; n++ {
		r, size := utf8.DecodeRuneInString(rest)
		rest = rest[size:]
		if k, ok := keyFor(r); ok {
			err = b.sendKey(ctx, t, k, 0, nil)
		} else {
			err = b.call(ctx, t.session, "Input.insertText", map[string]any{"text": string(r)}, nil)
		}
		if err != nil {
			return Tab{}, err
		}
		pause(25*time.Millisecond, 80*time.Millisecond)
	}
	if rest != "" {
		if err := b.call(ctx, t.session, "Input.insertText", map[string]any{"text": rest}, nil); err != nil {
			return Tab{}, err
		}
	}
	if submit {
		pause(150*time.Millisecond, 400*time.Millisecond)
		if err := b.sendKey(ctx, t, named["Enter"], 0, nil); err != nil {
			return Tab{}, err
		}
		pause(300*time.Millisecond, 600*time.Millisecond)
	}
	return b.describe(ctx, t), nil
}

// PressKey presses one key, with modifiers: "Enter", "Escape", "ArrowDown",
// "Control+a", "Mod+Shift+k" (Mod is Command on macOS, Control elsewhere).
func (b *Browser) PressKey(ctx context.Context, id, combo string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	parts := strings.Split(combo, "+")
	mods := 0
	for _, m := range parts[:len(parts)-1] {
		switch strings.ToLower(m) {
		case "alt", "option":
			mods |= modAlt
		case "control", "ctrl":
			mods |= modCtrl
		case "meta", "command", "cmd":
			mods |= modMeta
		case "shift":
			mods |= modShift
		case "mod":
			if runtime.GOOS == "darwin" {
				mods |= modMeta
			} else {
				mods |= modCtrl
			}
		default:
			return Tab{}, apperr.Newf("agent_bad_key", "unknown modifier %q", m)
		}
	}
	last := parts[len(parts)-1]
	k, ok := named[last]
	if !ok {
		r, size := utf8.DecodeRuneInString(last)
		if size != len(last) {
			return Tab{}, apperr.Newf("agent_bad_key", "unknown key %q", last)
		}
		if k, ok = keyFor(r); !ok {
			return Tab{}, apperr.Newf("agent_bad_key", "unknown key %q", last)
		}
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if err := b.sendKey(ctx, t, k, mods, nil); err != nil {
		return Tab{}, err
	}
	pause(200*time.Millisecond, 400*time.Millisecond)
	return b.describe(ctx, t), nil
}

// Scroll brings ref into view, or turns the mouse wheel by pages screens
// (negative is up) over the middle of the page.
func (b *Browser) Scroll(ctx context.Context, id, ref string, pages float64) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if ref != "" {
		if _, _, err := b.point(ctx, t, ref); err != nil {
			return Tab{}, err
		}
		return b.describe(ctx, t), nil
	}
	if pages == 0 {
		pages = 1
	}
	var m struct {
		CSSLayoutViewport struct {
			ClientWidth  float64 `json:"clientWidth"`
			ClientHeight float64 `json:"clientHeight"`
		} `json:"cssLayoutViewport"`
	}
	if err := b.call(ctx, t.session, "Page.getLayoutMetrics", nil, &m); err != nil {
		return Tab{}, err
	}
	w, h := m.CSSLayoutViewport.ClientWidth, m.CSSLayoutViewport.ClientHeight
	total := pages * h * 0.85
	const step = 120.0
	for done := 0.0; done < abs(total); done += step {
		d := min(step, abs(total)-done)
		if total < 0 {
			d = -d
		}
		if err := b.mouse(ctx, t, "mouseWheel", w/2, h/2, map[string]any{"deltaX": 0, "deltaY": d}); err != nil {
			return Tab{}, err
		}
		pause(15*time.Millisecond, 35*time.Millisecond)
	}
	pause(200*time.Millisecond, 300*time.Millisecond)
	return b.describe(ctx, t), nil
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
