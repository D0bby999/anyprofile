// SPDX-License-Identifier: AGPL-3.0-or-later

// apitoken prints the local API token of an installed AnyProfile, read from its
// secrets vault. Developer tool: it exists so a bench run can drive the real
// application instead of a copy of its launcher. The token is a secret; print
// it into a variable, never into a log or a transcript.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/secrets"
)

func main() {
	root := os.Getenv("ANYPROFILE_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		root = filepath.Join(home, "Library", "Application Support", "AnyProfile")
	}
	dirs := paths.Dirs{Root: root}
	v := secrets.NewVault(dirs.Vault(), secrets.Keychain{})
	tok, err := v.Get(secrets.ServiceAPIToken, "app")
	if err != nil {
		fmt.Fprintln(os.Stderr, "apitoken:", err)
		os.Exit(1)
	}
	fmt.Print(string(tok))
}
