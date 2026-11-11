//go:build !darwin

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

func initTray(_ *App, _ trayState) {}

func updateTrayState(_ trayState) {}

func stopTray() {}

func wakeTrayRunLoop() {}

