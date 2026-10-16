// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// ownerOnlyFiles has nothing to do on Windows: the data folder is under the
// user's %AppData%, whose inherited ACL already admits only the user,
// SYSTEM and administrators.
func ownerOnlyFiles() {}
