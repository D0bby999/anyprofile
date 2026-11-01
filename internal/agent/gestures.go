// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"time"
)

// Hover moves the mouse to an element and waits a moment, so hover menus open.
func (b *Browser) Hover(ctx context.Context, id, ref string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if err := b.ensureClickable(ctx, t, ref); err != nil {
		return Tab{}, err
	}
	x, y, err := b.point(ctx, t, ref)
	if err != nil {
		return Tab{}, err
	}
	for i := 1; i <= 3; i++ {
		f := float64(i) / 3
		if err := b.mouse(ctx, t, "mouseMoved", x*f, y*f, nil); err != nil {
			return Tab{}, err
		}
	}
	pause(150*time.Millisecond, 300*time.Millisecond)
	return b.describe(ctx, t), nil
}

// DoubleClick sends two clicks with clickCount 1 then 2: Chromium dispatches
// dblclick on the second one.
func (b *Browser) DoubleClick(ctx context.Context, id, ref string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if err := b.ensureClickable(ctx, t, ref); err != nil {
		return Tab{}, err
	}
	x, y, err := b.point(ctx, t, ref)
	if err != nil {
		return Tab{}, err
	}
	for _, n := range []int{1, 2} {
		if err := b.mouse(ctx, t, "mousePressed", x, y, map[string]any{"button": "left", "buttons": 1, "clickCount": n}); err != nil {
			return Tab{}, err
		}
		if err := b.mouse(ctx, t, "mouseReleased", x, y, map[string]any{"button": "left", "buttons": 0, "clickCount": n}); err != nil {
			return Tab{}, err
		}
		if n == 1 {
			pause(40*time.Millisecond, 90*time.Millisecond)
		}
	}
	pause(250*time.Millisecond, 500*time.Millisecond)
	return b.describe(ctx, t), nil
}

// RightClick opens the context menu at an element.
func (b *Browser) RightClick(ctx context.Context, id, ref string) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if err := b.ensureClickable(ctx, t, ref); err != nil {
		return Tab{}, err
	}
	x, y, err := b.point(ctx, t, ref)
	if err != nil {
		return Tab{}, err
	}
	if err := b.mouse(ctx, t, "mouseMoved", x, y, nil); err != nil {
		return Tab{}, err
	}
	if err := b.mouse(ctx, t, "mousePressed", x, y, map[string]any{"button": "right", "buttons": 2, "clickCount": 1}); err != nil {
		return Tab{}, err
	}
	if err := b.mouse(ctx, t, "mouseReleased", x, y, map[string]any{"button": "right", "buttons": 0, "clickCount": 1}); err != nil {
		return Tab{}, err
	}
	pause(150*time.Millisecond, 300*time.Millisecond)
	return b.describe(ctx, t), nil
}

// Drag moves the mouse from one element or point to another with the left
// button held. HTML5 drag-and-drop may need DOM drag events; ordinary
// mouse-driven drags (sliders, canvas, sortable lists) work this way.
func (b *Browser) Drag(ctx context.Context, id, fromRef, toRef string, x1, y1, x2, y2, steps int) (Tab, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return Tab{}, err
	}
	if err := b.activate(ctx, t); err != nil {
		return Tab{}, err
	}
	if fromRef != "" {
		x1f, y1f, err := b.point(ctx, t, fromRef)
		if err != nil {
			return Tab{}, err
		}
		x1, y1 = int(x1f), int(y1f)
	}
	if toRef != "" {
		x2f, y2f, err := b.point(ctx, t, toRef)
		if err != nil {
			return Tab{}, err
		}
		x2, y2 = int(x2f), int(y2f)
	}
	if steps <= 0 || steps > 60 {
		steps = 12
	}
	fx, fy := float64(x1), float64(y1)
	tx, ty := float64(x2), float64(y2)
	if err := b.mouse(ctx, t, "mouseMoved", fx, fy, nil); err != nil {
		return Tab{}, err
	}
	if err := b.mouse(ctx, t, "mousePressed", fx, fy, map[string]any{"button": "left", "buttons": 1, "clickCount": 1}); err != nil {
		return Tab{}, err
	}
	for i := 1; i <= steps; i++ {
		f := float64(i) / float64(steps)
		if err := b.mouse(ctx, t, "mouseMoved", fx+(tx-fx)*f, fy+(ty-fy)*f, map[string]any{"buttons": 1}); err != nil {
			return Tab{}, err
		}
		pause(8*time.Millisecond, 18*time.Millisecond)
	}
	if err := b.mouse(ctx, t, "mouseReleased", tx, ty, map[string]any{"button": "left", "buttons": 0, "clickCount": 1}); err != nil {
		return Tab{}, err
	}
	pause(200*time.Millisecond, 400*time.Millisecond)
	return b.describe(ctx, t), nil
}
