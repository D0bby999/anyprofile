// SPDX-License-Identifier: AGPL-3.0-or-later

// builtin-release emits a published platform pin for integration-test fixtures.
// It does not sign metadata, publish, install, or admit an unpublished candidate.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/anybrowser"
)

func writeRelease(out io.Writer, release engine.Release) error {
	if err := release.Validate(); err != nil {
		return err
	}
	if strings.Contains(release.URL, ".invalid/") || strings.Trim(release.SHA256, "0") == "" || len(release.Files) == 0 {
		return fmt.Errorf("builtin is unpublished; supply an independently verified candidate descriptor")
	}
	for file, hash := range release.Files {
		if len(hash) != 64 || strings.Trim(hash, "0") == "" {
			return fmt.Errorf("builtin core hash is not pinned: %s", file)
		}
	}
	return json.NewEncoder(out).Encode(release)
}

func main() {
	if err := writeRelease(os.Stdout, anybrowser.Builtin()[0]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
