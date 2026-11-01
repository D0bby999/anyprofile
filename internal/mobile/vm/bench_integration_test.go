// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin && vmintegration

// Benchmark for one headless device: create, boot, idle cost, screenshot,
// stop, disk. Run with:
//
//	ANYPROFILE_MOBILE_RUNTIME=<runtime> go test -tags vmintegration \
//	  -run TestBench -v -timeout 30m ./internal/mobile/vm/
//
// The test prints one line starting with "BENCH " (JSON) so a script can
// collect several runs. It never touches a user device: everything lives in
// t.TempDir() and is deleted before the test ends.

package vm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile"
)

const (
	benchSettle = 10 * time.Second
	benchSample = 2 * time.Second
	benchCount  = 30 // 60 s of idle sampling
)

type benchResult struct {
	Device       string  `json:"device"`
	Image        string  `json:"image"`
	Host         string  `json:"host"`
	MacOS        string  `json:"macos"`
	CreateMS     int64   `json:"create_ms"`
	BootMS       int64   `json:"boot_ms"`
	IdleCPUPct   float64 `json:"idle_cpu_pct"`
	RSSMB        float64 `json:"rss_mb"`
	PeakRSSMB    float64 `json:"peak_rss_mb"`
	Screenshot   int64   `json:"screenshot_ms"`
	Screen       string  `json:"screen"`
	PausedCPUPct float64 `json:"paused_cpu_pct"`
	ResumeMS     int64   `json:"resume_ms"`
	StopMS       int64   `json:"stop_ms"`
	DiskMB       float64 `json:"disk_mb"`
	IdleSeconds  float64 `json:"idle_seconds"`
}

func TestBench(t *testing.T) {
	rtDir := integrationRuntime(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	drv := New(rtDir, t.TempDir(), log)
	drv.SetFastStart(false) // the benchmark measures boot, not a thaw
	if err := drv.Available(); err != nil {
		t.Fatal(err)
	}

	id := benchID()
	cpus := 4
	if v := os.Getenv("ANYPROFILE_BENCH_CPUS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cpus = n
		}
	}
	d := mobile.Device{ID: id, Name: "Bench", RAMMB: 3072, CPUs: cpus}
	done := false
	defer func() {
		if !done {
			stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = drv.Stop(stopCtx, d)
			cancel()
		}
		if err := drv.Delete(context.Background(), d); err != nil {
			t.Logf("delete %s: %v", id, err)
		}
	}()

	started := time.Now()
	ref, image, err := drv.Create(ctx, d)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	d.Ref = ref
	createMS := time.Since(started).Milliseconds()

	run, err := drv.Start(ctx, d, mobile.StartOptions{Headless: true})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	metrics := drv.Metrics(id)
	if metrics.BootMS == 0 {
		t.Fatalf("no boot measurement")
	}

	a, ok := drv.ADB(d)
	if !ok {
		t.Fatalf("no adb after start")
	}

	// Let Android finish its post-boot work, then sample the idle cost.
	time.Sleep(benchSettle)
	firstSample, err := processSampleOf(run.Launch.PID)
	if err != nil {
		t.Fatalf("sample cpu: %v", err)
	}
	first := firstSample.cpuSeconds
	firstAt := time.Now()
	var last processSample
	var peak float64
	for i := 0; i < benchCount; i++ {
		s, err := processSampleOf(run.Launch.PID)
		if err != nil {
			t.Fatalf("sample cpu: %v", err)
		}
		if s.rssMB > peak {
			peak = s.rssMB
		}
		last = s
		time.Sleep(benchSample)
	}
	idleFor := time.Since(firstAt).Seconds()
	idlePct := (last.cpuSeconds - first) / idleFor * 100
	state, _ := a.Shell(ctx, "dumpsys power | grep -m1 'mWakefulness='")

	// Freeze the vCPUs and sample again: a paused device is the headless
	// steady state (the service pauses after 5 quiet minutes).
	if err := drv.Pause(id); err != nil {
		t.Fatalf("pause: %v", err)
	}
	time.Sleep(2 * time.Second)
	pFirst, err := processSampleOf(run.Launch.PID)
	if err != nil {
		t.Fatalf("sample paused cpu: %v", err)
	}
	pFirstAt := time.Now()
	var pLast processSample
	for i := 0; i < 15; i++ {
		pLast, err = processSampleOf(run.Launch.PID)
		if err != nil {
			t.Fatalf("sample paused cpu: %v", err)
		}
		time.Sleep(benchSample)
	}
	pausedPct := (pLast.cpuSeconds - pFirst.cpuSeconds) / time.Since(pFirstAt).Seconds() * 100
	if pausedPct > 1 {
		t.Logf("paused device still uses %.1f%% CPU", pausedPct)
	}
	resumeStart := time.Now()
	if err := drv.Resume(id); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if _, err := a.Shell(ctx, "echo ok"); err != nil {
		t.Fatalf("adb after resume: %v", err)
	}
	resumeMS := time.Since(resumeStart).Milliseconds()
	// Wake and let the frame settle before the screenshot.
	_, _ = a.Shell(ctx, "input keyevent KEYCODE_WAKEUP")
	time.Sleep(400 * time.Millisecond)

	// Screenshot cost: three runs, report the median.
	var shot []int64
	for i := 0; i < 3; i++ {
		t0 := time.Now()
		png, err := a.Output(ctx, "exec-out", "screencap", "-p")
		if err != nil {
			t.Fatalf("screenshot: %v", err)
		}
		if len(png) < 8 || string(png[1:4]) != "PNG" {
			t.Fatalf("screenshot is not a PNG (%d bytes)", len(png))
		}
		shot = append(shot, time.Since(t0).Milliseconds())
	}
	screenshotMS := median(shot)

	stopStart := time.Now()
	if err := drv.Stop(ctx, d); err != nil {
		t.Fatalf("stop: %v", err)
	}
	stopMS := time.Since(stopStart).Milliseconds()
	done = true

	// Nothing may outlive the stop.
	time.Sleep(500 * time.Millisecond)
	if pid, _ := pgrep("anyprofile-" + id); pid != 0 {
		t.Fatalf("qemu for %s still running as %d", id, pid)
	}

	disk := dirSizeMB(drv.dir(d))
	res := benchResult{
		Device:       id,
		Image:        image,
		Host:         strings.TrimSpace(runOutput("uname", "-m")),
		MacOS:        strings.TrimSpace(runOutput("sw_vers", "-productVersion")),
		CreateMS:     createMS,
		BootMS:       metrics.BootMS,
		IdleCPUPct:   round2(idlePct),
		RSSMB:        round2(last.rssMB),
		PeakRSSMB:    round2(peak),
		Screenshot:   screenshotMS,
		PausedCPUPct: round2(pausedPct),
		ResumeMS:     resumeMS,
		StopMS:       stopMS,
		DiskMB:       round2(disk),
		IdleSeconds:  round2(idleFor),
		Screen:       strings.TrimSpace(state),
	}
	line, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("BENCH %s\n", line)
	if out := os.Getenv("ANYPROFILE_BENCH_OUT"); out != "" {
		f, err := os.OpenFile(out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(string(line) + "\n")
			_ = f.Close()
		}
	}
	t.Logf("idle %.1f%% rss %.0f MB peak %.0f MB boot %d ms disk %.0f MB",
		res.IdleCPUPct, res.RSSMB, res.PeakRSSMB, res.BootMS, res.DiskMB)
}

func benchID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "bench" + strconv.FormatInt(time.Now().UnixNano()%1e11, 10)
	}
	return "bench" + hex.EncodeToString(b)[:11]
}

type processSample struct {
	cpuSeconds float64
	rssMB      float64
}

// processSampleOf returns the cumulative CPU time (seconds) and RSS (MB) of a
// process.
func processSampleOf(pid int) (processSample, error) {
	out, err := exec.Command("ps", "-o", "time=,rss=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return processSample{}, err
	}
	f := strings.Fields(string(out))
	if len(f) < 2 {
		return processSample{}, fmt.Errorf("ps: %q", out)
	}
	sec, err := clockSeconds(f[0])
	if err != nil {
		return processSample{}, err
	}
	kb, err := strconv.ParseFloat(f[1], 64)
	if err != nil {
		return processSample{}, err
	}
	return processSample{cpuSeconds: sec, rssMB: kb / 1024}, nil
}

// clockSeconds parses ps time: "SS.cc", "MM:SS.cc" or "HH:MM:SS.cc".
func clockSeconds(s string) (float64, error) {
	parts := strings.Split(s, ":")
	var total float64
	for i, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, err
		}
		switch len(parts) - i {
		case 1:
			total += v
		case 2:
			total += v * 60
		case 3:
			total += v * 3600
		}
	}
	return total, nil
}

func median(v []int64) int64 {
	if len(v) == 0 {
		return 0
	}
	c := append([]int64(nil), v...)
	for i := range c {
		for j := i + 1; j < len(c); j++ {
			if c[j] < c[i] {
				c[i], c[j] = c[j], c[i]
			}
		}
	}
	return c[len(c)/2]
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func pgrep(pattern string) (int, error) {
	out, err := exec.Command("pgrep", "-f", pattern).Output()
	if err != nil {
		return 0, err
	}
	lines := strings.Fields(string(out))
	if len(lines) == 0 {
		return 0, nil
	}
	return strconv.Atoi(lines[0])
}

func dirSizeMB(dir string) float64 {
	out, err := exec.Command("du", "-sk", dir).Output()
	if err != nil {
		return 0
	}
	f := strings.Fields(string(out))
	if len(f) == 0 {
		return 0
	}
	kb, _ := strconv.ParseFloat(f[0], 64)
	return kb / 1024
}

func runOutput(name string, args ...string) string {
	out, _ := exec.Command(name, args...).Output()
	return string(out)
}
