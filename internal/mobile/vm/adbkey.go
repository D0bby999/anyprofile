// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin

package vm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ensureADBKey creates AnyProfile's own ADB RSA keypair once inside runtimeDir
// (using adb keygen) so devices with ro.secure=1 and ro.debuggable=0 still
// accept AnyProfile's ADB/scrcpy connections after pre-authorizing the public key.
func ensureADBKey(ctx context.Context, adbBin, runtimeDir string) (privPath, pubKey string, err error) {
	privPath = filepath.Join(runtimeDir, "adbkey")
	pubPath := privPath + ".pub"
	if b, rerr := os.ReadFile(pubPath); rerr == nil && len(strings.TrimSpace(string(b))) > 0 {
		if _, serr := os.Stat(privPath); serr == nil {
			return privPath, strings.TrimSpace(string(b)), nil
		}
	}
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return "", "", err
	}
	os.Remove(privPath)
	os.Remove(pubPath)
	if out, err := exec.CommandContext(ctx, adbBin, "keygen", privPath).CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("adb keygen: %v: %s", err, strings.TrimSpace(string(out)))
	}
	_ = os.Chmod(privPath, 0o600)
	_ = os.Chmod(pubPath, 0o600)
	b, err := os.ReadFile(pubPath)
	if err != nil {
		return "", "", err
	}
	return privPath, strings.TrimSpace(string(b)), nil
}
