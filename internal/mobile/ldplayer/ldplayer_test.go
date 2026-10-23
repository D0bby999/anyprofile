// SPDX-License-Identifier: AGPL-3.0-or-later

package ldplayer

import (
	"context"
	"encoding/xml"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/D0bby999/anyprofile/internal/mobile"
	"github.com/D0bby999/anyprofile/internal/mobile/adb"
)

func TestParseList2(t *testing.T) {
	out := "0,LDPlayer,0,0,0,-1,-1,960,540,240\n1,ap-abc,393774,327922,1,4512,4600,1080,2400,420\nbad line\n"
	all := parseList2(out)
	if len(all) != 2 {
		t.Fatalf("%+v", all)
	}
	if all[1] != (Instance{Index: 1, Name: "ap-abc", Started: true, PID: 4512}) || all[0].Started {
		t.Fatalf("%+v", all)
	}
	if adbPort(1) != 5557 {
		t.Fatal(adbPort(1))
	}
}

func TestPick(t *testing.T) {
	have := map[string]bool{`D:\LD\ldconsole.exe`: true, `D:\LD\adb.exe`: true, `C:\x\ldconsole.exe`: true}
	in, err := pick([]string{`C:\x`, `D:\LD`}, func(p string) bool { return have[strings.ReplaceAll(p, "/", `\`)] })
	if err != nil || in.Dir != `D:\LD` {
		t.Fatalf("%+v %v", in, err)
	}
	if _, err := pick([]string{`C:\x`}, func(string) bool { return false }); !errors.Is(err, errMissing) {
		t.Fatalf("%v", err)
	}
}

func TestTunnelPrefs(t *testing.T) {
	x := tunnelPrefsXML(51234)
	var m struct {
		Bools []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:"value,attr"`
		} `xml:"boolean"`
		Strs []struct {
			Name string `xml:"name,attr"`
			Text string `xml:",chardata"`
		} `xml:"string"`
	}
	if err := xml.Unmarshal([]byte(x), &m); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(x, `<int name="P0.SocksPort" value="51234" />`) {
		t.Fatalf("port: %s", x)
	}
	want := map[string]string{"Enable": "true", "P0.Global": "true", "P0.RemoteDNS": "true", "P0.UdpInTcp": "false", "P0.Ipv6": "true"}
	for _, b := range m.Bools {
		if v, ok := want[b.Name]; ok && v != b.Value {
			t.Errorf("%s = %s", b.Name, b.Value)
		}
		delete(want, b.Name)
	}
	if len(want) > 0 {
		t.Errorf("missing %v", want)
	}
	for _, s := range m.Strs {
		if s.Name == "P0.SocksAddr" && s.Text != "127.0.0.1" {
			t.Errorf("addr %q", s.Text)
		}
		if (s.Name == "P0.SocksUser" || s.Name == "P0.SocksPass") && s.Text != "" {
			t.Error("the relay needs no credentials; the proxy's never reach Android")
		}
	}
	line := tunnelSetup(tmpPrefs)
	if strings.Contains(line, "'") {
		t.Fatal("asRoot wraps lines in single quotes")
	}
	for _, w := range []string{"always_on_vpn_app hev.sockstun", "always_on_vpn_lockdown 1", "ACTIVATE_VPN allow", "am force-stop hev.sockstun"} {
		if !strings.Contains(line, w) {
			t.Errorf("missing %q", w)
		}
	}
	if strings.Index(line, "am force-stop") > strings.Index(line, "cp ") {
		t.Error("settings are written while the app may still run")
	}
}

// fakeConsole is ldconsole with instances in memory.
type fakeConsole struct {
	mu      sync.Mutex
	list    []Instance
	running map[string]bool
	calls   [][]string
}

func (f *fakeConsole) run(ctx context.Context, bin string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, args)
	name := ""
	if i := slices.Index(args, "--name"); i >= 0 && i+1 < len(args) {
		name = args[i+1]
	}
	switch args[0] {
	case "list2":
		var b strings.Builder
		for _, in := range f.list {
			s := "0"
			if f.running[in.Name] {
				s = "1"
			}
			b.WriteString(strings.Join([]string{strconv.Itoa(in.Index), in.Name, "0", "0", s, strconv.Itoa(in.PID), "0"}, ",") + "\n")
		}
		return b.String(), nil
	case "add":
		f.list = append(f.list, Instance{Index: len(f.list), Name: name, PID: 100 + len(f.list)})
	case "remove":
		f.list = slices.DeleteFunc(f.list, func(in Instance) bool { return in.Name == name })
	case "isrunning":
		if f.running[name] {
			return "running", nil
		}
		return "stop", nil
	case "quit":
		f.running[name] = false
	case "launch":
		f.running[name] = true
	}
	return "", nil
}

func (f *fakeConsole) called(verb string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c[0] == verb {
			return c
		}
	}
	return nil
}

func newFake(t *testing.T) (*Driver, *fakeConsole) {
	t.Helper()
	fc := &fakeConsole{running: map[string]bool{}}
	d := New(t.TempDir(), nil)
	d.find = func() (Install, error) {
		return Install{Dir: "C:/LD", Console: "C:/LD/ldconsole.exe", ADB: "C:/LD/adb.exe"}, nil
	}
	d.run = fc.run
	return d, fc
}

func TestCreateNamesTheInstance(t *testing.T) {
	d, fc := newFake(t)
	ref, img, err := d.Create(context.Background(), mobile.Device{ID: "abc", RAMMB: 3072, CPUs: 4})
	if err != nil || ref != "ap-abc" || img != "ldplayer" {
		t.Fatalf("%q %q %v", ref, img, err)
	}
	m := fc.called("modify")
	if m == nil || !slices.Contains(m, "--root") || !slices.Contains(m, "3072") || slices.Contains(m, "--index") {
		t.Fatalf("modify %v", m)
	}
	if _, ok, _ := (console{run: fc.run}).find(context.Background(), "ap-abc"); !ok {
		t.Fatal("not added")
	}
}

// An instance deleted in LDPlayer, its index then reused: AnyProfile must not
// drive the other one.
func TestDeletedOutsideIsGone(t *testing.T) {
	d, fc := newFake(t)
	fc.list = []Instance{{Index: 0, Name: "someone-else"}}
	_, err := d.Start(context.Background(), mobile.Device{ID: "abc", Ref: "ap-abc"}, mobile.StartOptions{})
	if !errors.Is(err, errGone) {
		t.Fatalf("%v", err)
	}
	if fc.called("launch") != nil {
		t.Fatal("launched another instance")
	}
	if err := d.Delete(context.Background(), mobile.Device{ID: "abc", Ref: "ap-abc"}); err != nil {
		t.Fatalf("delete of a gone instance: %v", err)
	}
	if fc.called("remove") != nil {
		t.Fatal("removed another instance")
	}
}

func TestBackupRestoreByName(t *testing.T) {
	d, fc := newFake(t)
	dir := t.TempDir()
	dev := mobile.Device{ID: "abc", Ref: "ap-abc"}
	if err := d.Backup(context.Background(), dev, dir); err != nil {
		t.Fatal(err)
	}
	b := fc.called("backup")
	if !slices.Contains(b, "ap-abc") || !strings.HasSuffix(b[len(b)-1], backupFile) {
		t.Fatalf("backup %v", b)
	}
	if err := d.Restore(context.Background(), dev, dir); err == nil {
		t.Fatal("restored from a folder without a backup")
	}
}

func TestLocaleRefusesShellCode(t *testing.T) {
	// No adb is called for values that fail the checks.
	d, _ := newFake(t)
	if err := d.locale(context.Background(), adb.Client{Bin: "/nonexistent"}, "UTC; reboot", "en$(id)"); err != nil {
		t.Fatalf("%v", err)
	}
}
