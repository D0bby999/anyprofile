// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"math"
	"strings"
	"testing"
)

func TestLocationHelpers(t *testing.T) {
	if !ValidCoords(10.7769, 106.7009) || ValidCoords(95, 0) || ValidCoords(0, -190) {
		t.Fatal("ValidCoords failed")
	}
	lat1, lon1 := JitterCoords("dev-a", 10.7769, 106.7009)
	lat2, lon2 := JitterCoords("dev-b", 10.7769, 106.7009)
	if lat1 == lat2 && lon1 == lon2 {
		t.Fatal("different devices got identical jittered coords")
	}
	if math.Abs(lat1-10.7769) > 0.006 || math.Abs(lon1-106.7009) > 0.006 {
		t.Fatalf("jitter out of range: %f, %f", lat1, lon1)
	}
	s, err := LocationShellScript(lat1, lon1)
	if err != nil || !strings.Contains(s, "set-test-provider-location gps --location") {
		t.Fatalf("script = %q, err = %v", s, err)
	}
	if _, err := LocationShellScript(91, 0); err == nil {
		t.Fatal("accepted out-of-range latitude")
	}
}
