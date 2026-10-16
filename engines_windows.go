// SPDX-License-Identifier: AGPL-3.0-or-later

package main

// furyRuns is false: the Fury core reads its config on extra file
// descriptors, which Windows processes do not inherit. Apostate is the
// only engine on Windows.
const furyRuns = false

// bundledConfigRel is the fury-config helper next to AnyProfile.exe. It
// serves Apostate profiles too (locale per country).
const bundledConfigRel = "fury-config.exe"
