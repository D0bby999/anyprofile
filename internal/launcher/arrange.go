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

// ErrNoScreen: the area of the screen could not be read.
var ErrNoScreen = errors.New("could not read the size of the screen")

// grid cuts area into n cells, rows of up to ceil(sqrt(n)) cells.
func grid(area Rect, n int) []Rect {
	if n <= 0 {
		return nil
	}
	cols := int(math.Ceil(math.Sqrt(float64(n))))
	rows := (n + cols - 1) / cols
	w, h := area.W/cols, area.H/rows
	out := make([]Rect, n)
	for i := range out {
		out[i] = Rect{X: area.X + (i%cols)*w, Y: area.Y + (i/cols)*h, W: w, H: h}
	}
	return out
}

// Arrange puts the window of every open profile in a grid over the screen
// AnyProfile is on, oldest first, and brings those on the current desktop
// to the front. Only the user asks for it: a script or an agent never
// resizes a window. It returns how many windows were placed.
func (l *Launcher) Arrange(ctx context.Context) (int, error) {
	area, ok := workArea()
	if !ok {
		return 0, ErrNoScreen
	}
	type open struct {
		pid     int
		cdp     *cdppipe.Conn
		started time.Time
	}
	var ps []open
	l.mu.Lock()
	for _, p := range l.procs {
		if p.info.State == Running && p.cdp != nil {
			ps = append(ps, open{p.info.PID, p.cdp, p.started})
		}
	}
	l.mu.Unlock()
	sort.Slice(ps, func(i, j int) bool { return ps[i].started.Before(ps[j].started) })
	placed := 0
	for i, r := range grid(area, len(ps)) {
		if err := placeWindow(ctx, ps[i].cdp, r); err != nil {
			l.log.Warn("arrange window", "pid", ps[i].pid, "err", err)
			continue
		}
		placed++
	}
	for _, p := range ps {
		if onCurrentDesktop(p.pid) { // another desktop's window stays there: no jump
			activate(p.pid)
		}
	}
	return placed, nil
}

// placeWindow moves the window of the browser's first page to r.
func placeWindow(ctx context.Context, c *cdppipe.Conn, r Rect) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	ts, err := pageTargets(ctx, c)
	if err != nil {
		return err
	}
	if len(ts) == 0 {
		return errors.New("no window")
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
