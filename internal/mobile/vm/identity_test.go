// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strings"
	"testing"
)

func TestIdentityForDeterministicAndUnique(t *testing.T) {
	a := IdentityFor("dev-1", "")
	a2 := IdentityFor("dev-1", "")
	b := IdentityFor("dev-2", "")
	if a != a2 {
		t.Fatalf("not deterministic: %+v vs %+v", a, a2)
	}
	if a.Serial == b.Serial || len(a.Serial) != 14 {
		t.Fatalf("serials = %q, %q", a.Serial, b.Serial)
	}
	if a.MIDR == "" || !strings.HasPrefix(a.MIDR, "0x41") {
		t.Fatalf("MIDR should mask Apple CPU with ARM 0x41, got %q", a.MIDR)
	}
	p := IdentityFor("dev-1", "s24")
	if p.Model != "SM-S921B" || !ValidIMEI(p.IMEI) || p.GLRenderer == "" {
		t.Fatalf("template s24 got %+v", p)
	}
	for _, tpl := range []string{"pixel8", "pixel9", "s24", "s24ultra", "s23ultra", "a54", "xiaomi14pro", "xiaomi13ultra", "oneplus12", "oppofindx6pro"} {
		got := IdentityFor("dev-"+tpl, tpl)
		if got.Template != tpl || !ValidIMEI(got.IMEI) {
			t.Fatalf("template %s invalid: %+v", tpl, got)
		}
	}
}

func TestRenderIdentityScript(t *testing.T) {
	id := IdentityFor("dev-1", "pixel8")
	script, err := RenderIdentityScript(id)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"rp 'ro.product.model' 'Pixel 8'",
		"rp 'ro.build.tags' 'release-keys'",
		"rp 'ro.build.type' 'user'",
		"rp 'ro.debuggable' '0'",
		"rp 'ro.secure' '1'",
		"rp 'ro.boot.verifiedbootstate' 'green'",
		"rp 'ro.board.platform' 'zuma'",
		"rp 'ro.boot.hardware' 'shiba'",
		"rp 'ro.boot.baseband' 'g5300q-240919-241106-B-12612894'",
		"rp 'ro.build.product' 'shiba'",
		"rp 'ro.kernel.qemu' '0'",
		"rp --delete \"$p\"",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("missing %q in script", want)
		}
	}
	bad := id
	bad.Model = "Pixel 8; rm -rf /"
	if _, err := RenderIdentityScript(bad); err == nil {
		t.Fatal("accepted shell injection in model")
	}
}

func TestCarrierAndBatteryScripts(t *testing.T) {
	if s := CarrierShellScript("vn"); !strings.Contains(s, "Viettel") || !strings.Contains(s, "45204") {
		t.Fatalf("VN carrier = %q", s)
	}
	if s := CarrierShellScript("US"); !strings.Contains(s, "T-Mobile") || !strings.Contains(s, "310260") {
		t.Fatalf("US carrier = %q", s)
	}
	if s := CarrierShellScript("ZZ"); s != "" {
		t.Fatalf("unknown country should be empty, got %q", s)
	}
	bat := BatteryShellScript("dev-1")
	if !strings.Contains(bat, "dumpsys battery unplug") || !strings.Contains(bat, "set level") {
		t.Fatalf("battery = %q", bat)
	}
}

func TestPickDistinctTemplateAvoidsDuplicates(t *testing.T) {
	cands := []string{"a", "b", "c", "d"}
	used := map[string]int{}
	seen := map[string]bool{}
	for i := 0; i < len(cands); i++ {
		got := PickDistinctTemplate(string(rune('0'+i)), cands, used)
		if seen[got] {
			t.Fatalf("step %d picked duplicate %q before exhausting pool (used=%v)", i, got, used)
		}
		seen[got] = true
		used[got]++
	}
}

func TestIdentityFromProfile(t *testing.T) {
	props := map[string]string{
		"ro.product.brand":        "vivo",
		"ro.product.manufacturer": "vivo",
		"ro.product.model":        "V2409",
		"ro.product.device":       "PD2419",
		"ro.product.name":         "PD2419",
		"ro.build.fingerprint":    "vivo/PD2419/PD2419:15/AP3A.240905.015.A2/1728000000:user/release-keys",
	}
	id, ok := IdentityFromProfile("dev-99", "vivo-x200", props, "mt6991", "mt6991", "mt6991", "ARM", "Immortalis-G925")
	if !ok {
		t.Fatal("expected valid identity")
	}
	if id.Template != "vivo-x200" || id.Model != "V2409" || id.BuildID != "AP3A.240905.015.A2" || id.Incremental != "1728000000" {
		t.Fatalf("unexpected identity: %+v", id)
	}
	if !ValidIMEI(id.IMEI) || len(id.Serial) != 14 {
		t.Fatalf("invalid serial/imei: %q / %q", id.Serial, id.IMEI)
	}
}
