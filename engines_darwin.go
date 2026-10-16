// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// furyRuns: the Fury core reads its config on extra file descriptors, which
// only the macOS launcher provides.
const furyRuns = true

// bundledConfigRel is the fury-config helper inside the app bundle, relative
// to the executable. It also serves Apostate profiles (locale per country).
const bundledConfigRel = "../Resources/fury-config"
