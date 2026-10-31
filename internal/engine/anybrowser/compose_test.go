// SPDX-License-Identifier: AGPL-3.0-or-later

package anybrowser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/D0bby999/anyprofile/internal/paths"
)

func TestParseComposition(t *testing.T) {
	explain := func(cpu, mem string) string {
		return "anybrowser fingerprint composition\n\nsurface                   layer             evidence               value\n" +
			"gpu_identity              dispersion        catalogue-value        metal-apple-m4\n" +
			cpu + "\n" + mem + "\n" +
			"panel                     dispersion        physical-ground-truth  mbp16-default\n"
	}
	tests := []struct {
		name     string
		out      string
		servable bool
		cores    int
		mem      int
		wantErr  bool
	}{
		{
			name:     "servable chip pins its own cores and memory",
			out:      explain("cpu                       dispersion        catalogue-value        cores-8", "memory                    dispersion        catalogue-value        gib-16"),
			servable: true, cores: 8, mem: 16,
		},
		{
			name:     "twelve core chip",
			out:      explain("cpu                       dispersion        catalogue-value        cores-12", "memory                    dispersion        catalogue-value        gib-32"),
			servable: true, cores: 12, mem: 32,
		},
		{
			name: "unservable chip is left inherited",
			out:  explain("cpu                       host-inherited    host-inherited         (inherited)", "memory                    dispersion        catalogue-value        gib-16"),
		},
		{
			name:     "memory may stay inherited while cpu is served",
			out:      explain("cpu                       dispersion        catalogue-value        cores-8", "memory                    host-inherited    host-inherited         (inherited)"),
			servable: true, cores: 8, mem: 0,
		},
		{
			name:    "cpu row missing",
			out:     "anybrowser fingerprint composition\n",
			wantErr: true,
		},
		{
			name:    "unrecognised cpu value",
			out:     explain("cpu                       dispersion        catalogue-value        something-else", "memory                    dispersion        catalogue-value        gib-16"),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseComposition(tc.out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseComposition() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseComposition() error: %v", err)
			}
			if got.Servable != tc.servable || got.Cores != tc.cores || got.MemGiB != tc.mem {
				t.Fatalf("ParseComposition() = %+v, want servable=%v cores=%d mem=%d", got, tc.servable, tc.cores, tc.mem)
			}
		})
	}
}

func TestCompositionComplete(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    Composition
		want bool
	}{
		{"chip and memory served", Composition{Servable: true, Cores: 8, MemGiB: 16}, true},
		{"memory left to the host", Composition{Servable: true, Cores: 8}, false},
		{"chip left to the host", Composition{MemGiB: 16}, false},
		{"nothing served", Composition{}, false},
	} {
		if got := tc.c.Complete(); got != tc.want {
			t.Fatalf("%s: Complete() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDrawServable(t *testing.T) {
	t.Run("redraws past unservable seeds", func(t *testing.T) {
		seeds := []string{"a", "b", "c"}
		i := 0
		draw := func() string { s := seeds[i]; i++; return s }
		compose := func(seed string) (Composition, error) {
			if seed == "c" {
				return Composition{Servable: true, Cores: 8, MemGiB: 16}, nil
			}
			return Composition{}, nil
		}
		seed, comp, ok, err := drawServable(draw, compose, 5)
		if err != nil || !ok || seed != "c" || comp.Cores != 8 {
			t.Fatalf("drawServable() = %q %+v %v %v, want c cores=8 true nil", seed, comp, ok, err)
		}
		if i != 3 {
			t.Fatalf("drew %d seeds, want 3", i)
		}
	})

	t.Run("stops when the engine cannot be run", func(t *testing.T) {
		drawn := 0
		draw := func() string { drawn++; return "a" }
		compose := func(string) (Composition, error) { return Composition{}, errTest }
		if _, _, ok, err := drawServable(draw, compose, 5); ok || !errors.Is(err, errTest) {
			t.Fatalf("drawServable() ok=%v err=%v, want false and the engine error", ok, err)
		}
		if drawn != 1 {
			t.Fatalf("drew %d seeds, want 1", drawn)
		}
	})

	t.Run("gives up after the attempt budget", func(t *testing.T) {
		drawn := 0
		draw := func() string { drawn++; return "a" }
		compose := func(string) (Composition, error) { return Composition{}, nil }
		if _, _, ok, err := drawServable(draw, compose, 3); ok || err != nil {
			t.Fatalf("drawServable() ok=%v err=%v, want false and nil", ok, err)
		}
		if drawn != 3 {
			t.Fatalf("drew %d seeds, want 3", drawn)
		}
	})

	t.Run("an empty draw is not a seed", func(t *testing.T) {
		if _, _, ok, err := drawServable(func() string { return "" }, func(string) (Composition, error) {
			t.Fatal("compose called with an empty seed")
			return Composition{}, nil
		}, 1); ok || err != nil {
			t.Fatalf("drawServable() ok=%v err=%v, want false and nil", ok, err)
		}
	})
}

type testError struct{}

func (testError) Error() string { return "engine not installed" }

var errTest = testError{}

// TestParseCompositionRecordedExplain fails when the engine's explain format
// changes: the recorded outputs come from the real binary (seeds 3e8 and
// 818a8d763363c626), one served and one left to the host.
func TestParseCompositionRecordedExplain(t *testing.T) {
	for _, tc := range []struct {
		file     string
		servable bool
		cores    int
	}{
		{"testdata/explain-unservable.txt", false, 0},
		{"testdata/explain-servable.txt", true, 8},
	} {
		t.Run(tc.file, func(t *testing.T) {
			out, err := os.ReadFile(tc.file)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseComposition(string(out))
			if err != nil {
				t.Fatal(err)
			}
			if got.Servable != tc.servable {
				t.Fatalf("Servable = %v, want %v", got.Servable, tc.servable)
			}
			if tc.cores > 0 && got.Cores != tc.cores {
				t.Fatalf("Cores = %d, want %d", got.Cores, tc.cores)
			}
		})
	}
}

// TestComposeIdentityRefusesWithoutTheEngine covers the welcome screen: a
// profile must not be created before the engine is installed, because nothing
// can check the seed and the host machine would silently become the profile's.
func TestComposeIdentityRefusesWithoutTheEngine(t *testing.T) {
	a := New(paths.Dirs{Root: t.TempDir()})
	a.host = func() (int, int) { return 8, 16 }
	_, err := a.ComposeIdentity(context.Background(), a.pinned.Version, 0)
	if err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("ComposeIdentity() error = %v, want an install refusal", err)
	}
}

// TestComposeIdentityFailsClosedWhenExplainFails covers an installed engine that
// cannot answer: profile creation must refuse instead of pinning the host
// machine, which is the impossible-machine bug this change removes.
func TestComposeIdentityFailsClosedWhenExplainFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell script as the fake binary")
	}
	a := New(paths.Dirs{Root: t.TempDir()})
	a.host = func() (int, int) { return 8, 16 }
	bin := filepath.Join(a.dir(a.pinned.Version), filepath.FromSlash(a.pinned.Bin))
	if err := os.MkdirAll(filepath.Dir(bin), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho boom >&2\nexit 3\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ComposeIdentity(context.Background(), a.pinned.Version, 0); err == nil {
		t.Fatal("ComposeIdentity() succeeded with a broken engine, want a refusal")
	}
}
