// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "syscall"

// ownerOnlyFiles makes everything this process creates owner-only, including
// SQLite's -wal/-shm files, which hold database pages, and the browsers it
// spawns.
func ownerOnlyFiles() { syscall.Umask(0o077) }
