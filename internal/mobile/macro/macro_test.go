// SPDX-License-Identifier: AGPL-3.0-or-later

package macro

import (
	"context"
	"testing"
)

type fakePlayer struct {
	w, h  int
	count int
}

func (f *fakePlayer) ScreenSize() (int, int)                 { return f.w, f.h }
func (f *fakePlayer) Tap(_ context.Context, _, _ int) error  { f.count++; return nil }
func (f *fakePlayer) Swipe(_ context.Context, _, _, _, _, _ int) error {
	f.count++
	return nil
}
func (f *fakePlayer) PressKey(_ context.Context, _ string) error { f.count++; return nil }

func TestMacroRecordSaveAndReplay(t *testing.T) {
	st := Store{Dir: t.TempDir()}
	var rec Recorder
	rec.Start("dev1", "Scroll TikTok")
	rec.Record("dev1", Step{Kind: "tap", RX: 0.5, RY: 0.5})
	rec.Record("dev1", Step{Kind: "swipe", RX: 0.5, RY: 0.8, RX2: 0.5, RY2: 0.2, DurationMS: 200})
	rec.Record("other", Step{Kind: "key", Key: "back"}) // ignored: wrong device
	rec.Record("dev1", Step{Kind: "key", Key: "home"})

	sc, ok := rec.Stop("m1")
	if !ok || len(sc.Steps) != 3 || sc.Name != "Scroll TikTok" {
		t.Fatalf("stopped = %+v, ok=%v", sc, ok)
	}
	if err := st.Save(sc); err != nil {
		t.Fatal(err)
	}
	list, err := st.List()
	if err != nil || len(list) != 1 || list[0].ID != "m1" {
		t.Fatalf("list = %+v, err=%v", list, err)
	}
	loaded, err := st.Load("m1")
	if err != nil {
		t.Fatal(err)
	}
	fp := &fakePlayer{w: 780, h: 1688}
	if err := Play(context.Background(), loaded, fp, PlayOptions{Loops: 2, Speed: 10}); err != nil {
		t.Fatal(err)
	}
	if fp.count != 6 {
		t.Fatalf("played %d steps, want 6", fp.count)
	}
	if err := st.Delete("m1"); err != nil {
		t.Fatal(err)
	}
}
