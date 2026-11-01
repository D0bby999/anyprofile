// SPDX-License-Identifier: AGPL-3.0-or-later

package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/D0bby999/anyprofile/internal/cdppipe"
)

// Rect is an area of the screen in points (DIPs), from the top left of the
// main screen, as Chromium's window bounds are.
type Rect struct{ X, Y, W, H int }

// screenArea is workArea; tests set their own, as reading the screen waits
// for the app's main thread, which a test does not run.
var screenArea = workArea

// ErrNoScreen: the area of the screen could not be read.
var ErrNoScreen = errors.New("could not read the size of the screen")

// cellGap is the strip left between two arranged windows, so their edges stay
// apart when both are focused and the shadows do not overlap.
const cellGap = 8

// A pane smaller than this cannot show a browser's tab strip and toolbar over
// a usable page: below it, arranging the windows on open is skipped. A 1440
// point wide screen still tiles six windows (474x402) and stops at seven.
const (
	minTileW = 460
	minTileH = 340
)

// grid cuts area into n cells, rows of up to ceil(sqrt(n)) cells, with cellGap
// between them.
func grid(area Rect, n int) []Rect {
	if n <= 0 {
		return nil
	}
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	rows := (n + cols - 1) / cols
	w := (area.W - cellGap*(cols-1)) / cols
	h := (area.H - cellGap*(rows-1)) / rows
	out := make([]Rect, n)
	for i := range out {
		out[i] = Rect{X: area.X + (i%cols)*(w+cellGap), Y: area.Y + (i/cols)*(h+cellGap), W: w, H: h}
	}
	return out
}

// Arrange puts the window of every open profile in a grid over the screen
// AnyProfile is on, in the order of the profile IDs in order (as the list
// shows them; others after, oldest first). With raise, the windows on the
// current desktop come to the front and the one in the first cell keeps the
// focus. Only the user asks for it: a script or an agent never resizes a
// window. It returns how many windows were placed.
func (l *Launcher) Arrange(ctx context.Context, order []string, raise bool) (int, error) {
	return l.arrange(ctx, order, nil, raise)
}

// Tile lays out the windows of just these profiles, in this order, and leaves
// the first in front: what opening several at once does, so the new windows
// neither land all on the same spot nor take the focus from each other.
func (l *Launcher) Tile(ctx context.Context, ids []string) (int, error) {
	return l.arrange(ctx, ids, ids, true)
}

// arrange places the windows of the running profiles in order; only narrows it
// to those IDs (nil means every running profile). raise brings them in front.
func (l *Launcher) arrange(ctx context.Context, order, only []string, raise bool) (int, error) {
	area, ok := screenArea()
	if !ok {
		return 0, ErrNoScreen
	}
	type open struct {
		pid     int
		cdp     *cdppipe.Conn
		started time.Time
		place   int
	}
	at := make(map[string]int, len(order))
	for i, id := range order {
		at[id] = i
	}
	wanted := make(map[string]bool, len(only))
	for _, id := range only {
		wanted[id] = true
	}
	var ps []open
	l.mu.Lock()
	for id, p := range l.procs {
		if only != nil && !wanted[id] {
			continue
		}
		if p.info.State == Running && p.cdp != nil {
			place, ok := at[id]
			if !ok {
				place = len(order)
			}
			ps = append(ps, open{p.info.PID, p.cdp, p.started, place})
		}
	}
	l.mu.Unlock()
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].place != ps[j].place {
			return ps[i].place < ps[j].place
		}
		return ps[i].started.Before(ps[j].started)
	})
	placed := 0
	cells := grid(area, len(ps))
	// Opening many at once lays them out only while every window still gets a
	// usable pane; past that a grid of slivers helps nobody, and the user can
	// still ask for it by hand.
	if only != nil && len(cells) > 0 && (cells[0].W < minTileW || cells[0].H < minTileH) {
		return 0, nil
	}
	front := -1 // the window in the first cell of this desktop: it ends up in front
	for i, r := range cells {
		if err := placeWindow(ctx, ps[i].cdp, r); err != nil {
			l.log.Warn("arrange window", "pid", ps[i].pid, "err", err)
			continue
		}
		if front < 0 && onCurrentDesktop(ps[i].pid) {
			front = i
		}
		placed++
	}
	if raise && front >= 0 {
		// Every window must rise above this app, so all of them are
		// activated, last cell first: the first window is left in front and
		// the focus does not land on whichever happened to be activated last.
		// A window on another desktop stays there: raising it would yank the
		// screen to that desktop.
		l.claimFront() // a launch still being kept behind must not undo this
		for i := len(ps) - 1; i >= 0; i-- {
			if onCurrentDesktop(ps[i].pid) {
				activate(ps[i].pid)
			}
		}
		activate(ps[front].pid)
	}
	return placed, nil
}

// windowWait is how long a browser just started gets to show its window;
// several starting at once on a slow computer take seconds.
const windowWait = 10 * time.Second

// placeWindow moves the window of the browser's first page to r.
func placeWindow(ctx context.Context, c *cdppipe.Conn, r Rect) error {
	ctx, cancel := context.WithTimeout(ctx, windowWait)
	defer cancel()
	// A browser just started may not show its window yet.
	ts, err := pageTargets(ctx, c)
	for err == nil && len(ts) == 0 {
		select {
		case <-ctx.Done():
			return errors.New("no window")
		case <-time.After(200 * time.Millisecond):
		}
		ts, err = pageTargets(ctx, c)
	}
	if err != nil {
		return err
	}
	raw, err := c.Call(ctx, "", "Browser.getWindowForTarget", map[string]any{"targetId": ts[0].TargetID})
	if err != nil {
		return err
	}
	var w struct {
		WindowID int `json:"windowId"`
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		return err
	}
	// A minimized, maximized or full-screen window must be normal first; the
	// state cannot change in the same call as the bounds.
	if _, err := c.Call(ctx, "", "Browser.setWindowBounds", map[string]any{
		"windowId": w.WindowID, "bounds": map[string]any{"windowState": "normal"}}); err != nil {
		return err
	}
	_, err = c.Call(ctx, "", "Browser.setWindowBounds", map[string]any{"windowId": w.WindowID,
		"bounds": map[string]any{"left": r.X, "top": r.Y, "width": r.W, "height": r.H}})
	return err
}
