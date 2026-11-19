// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/D0bby999/anyprofile/internal/mobile/adb"
)

// ValidCoords reports whether lat/lon are within WGS-84 bounds.
func ValidCoords(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) &&
		lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}

// JitterCoords offsets (lat, lon) deterministically per deviceID by up to
// ~0.005 degrees (~500 m) so multiple devices sharing a proxy city do not sit
// on the exact same GPS coordinate.
func JitterCoords(deviceID string, lat, lon float64) (float64, float64) {
	h := sha256.Sum256([]byte("anyprofile-mobile-gps:" + deviceID))
	u1 := float64(binary.LittleEndian.Uint32(h[0:4]))/float64(math.MaxUint32)*2 - 1
	u2 := float64(binary.LittleEndian.Uint32(h[4:8]))/float64(math.MaxUint32)*2 - 1
	jLat := math.Max(-89.9, math.Min(89.9, lat+u1*0.005))
	jLon := math.Max(-179.9, math.Min(179.9, lon+u2*0.005))
	return jLat, jLon
}

// LocationShellScript builds the Android 12+ `cmd location` script that
// enables location services and injects a realistic GPS fix.
func LocationShellScript(lat, lon float64) (string, error) {
	if !ValidCoords(lat, lon) {
		return "", fmt.Errorf("invalid coordinates: %f, %f", lat, lon)
	}
	return fmt.Sprintf(
		"settings put secure location_mode 3; "+
			"cmd location providers add-test-provider gps --supportsAltitude --requiresSatellite >/dev/null 2>&1 || true; "+
			"cmd location providers set-test-provider-enabled gps true >/dev/null 2>&1 || true; "+
			"cmd location providers set-test-provider-location gps --location %.6f,%.6f --accuracy 12.0",
		lat, lon,
	), nil
}

// ApplyLocation applies (lat, lon) on a running device via ADB.
func ApplyLocation(ctx context.Context, a adb.Client, lat, lon float64) error {
	script, err := LocationShellScript(lat, lon)
	if err != nil {
		return err
	}
	c, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err = a.Shell(c, script)
	return err
}
