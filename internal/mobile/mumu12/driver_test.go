// SPDX-License-Identifier: AGPL-3.0-or-later

package mumu12

import (
	"strings"
	"testing"
)

func TestParseInfoAndSettings(t *testing.T) {
	single := `{"index":1,"name":"ap-abc","adb_port":16416,"is_process_started":true,"is_android_started":true,"pid":4200}`
	infos, err := parseInfo(single)
	if err != nil || len(infos) != 1 || infos[0].ADBPort != 16416 || !infos[0].IsAndroidStarted {
		t.Fatalf("single = %+v, err=%v", infos, err)
	}
	multi := `{"0":{"name":"Main","adb_port":16384},"2":{"name":"ap-2","adb_port":16448}}`
	infos, err = parseInfo(multi)
	if err != nil || len(infos) != 2 {
		t.Fatalf("multi = %+v, err=%v", infos, err)
	}
	args := settingArgs(1, 4, 4096, 780, 1688, 320)
	line := strings.Join(args, " ")
	for _, want := range []string{"setting -v 1", "performance_cpu.custom -val 4", "performance_mem.custom -val 4",
		"resolution_width.custom -val 780", "resolution_height.custom -val 1688", "resolution_dpi.custom -val 320"} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %q in %s", want, line)
		}
	}
}
