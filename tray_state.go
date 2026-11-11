// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "fmt"

type trayState struct {
	Active        bool
	HasRunning    bool
	StatusLabel   string
	OpenLabel     string
	SettingsLabel string
	UpdatesLabel  string
	StopAllLabel  string
	QuitLabel     string
	Tooltip       string
}

type trayTextCatalog struct {
	Idle      string
	APISuffix string
	Open      string
	Settings  string
	Updates   string
	StopAll   string
	Quit      string
}

var trayText = map[string]trayTextCatalog{
	"en": {
		Idle:      "Status: Idle",
		APISuffix: " · API on",
		Open:      "Open AnyProfile",
		Settings:  "Settings…",
		Updates:   "Check for Updates…",
		StopAll:   "Stop All Sessions",
		Quit:      "Quit AnyProfile",
	},
	"vi": {
		Idle:      "Trạng thái: Đang chờ",
		APISuffix: " · API bật",
		Open:      "Mở AnyProfile",
		Settings:  "Cài đặt…",
		Updates:   "Kiểm tra bản cập nhật…",
		StopAll:   "Đóng tất cả phiên",
		Quit:      "Thoát AnyProfile",
	},
}

func formatRunningSummary(lang string, browsers, mobile int) string {
	if lang == "vi" {
		switch {
		case browsers > 0 && mobile > 0:
			return fmt.Sprintf("%d trình duyệt, %d mobile", browsers, mobile)
		case browsers > 0:
			return fmt.Sprintf("%d trình duyệt", browsers)
		default:
			return fmt.Sprintf("%d mobile", mobile)
		}
	}
	bUnit := "browsers"
	if browsers == 1 {
		bUnit = "browser"
	}
	switch {
	case browsers > 0 && mobile > 0:
		return fmt.Sprintf("%d %s, %d mobile", browsers, bUnit, mobile)
	case browsers > 0:
		return fmt.Sprintf("%d %s", browsers, bUnit)
	default:
		return fmt.Sprintf("%d mobile", mobile)
	}
}

func formatTrayState(lang string, browsers, mobile int, apiRunning bool) trayState {
	if browsers < 0 {
		browsers = 0
	}
	if mobile < 0 {
		mobile = 0
	}
	tx, ok := trayText[lang]
	if !ok {
		lang = "en"
		tx = trayText["en"]
	}

	hasRunning := browsers+mobile > 0
	status := tx.Idle
	tooltip := "AnyProfile"
	if hasRunning {
		summary := formatRunningSummary(lang, browsers, mobile)
		if lang == "vi" {
			status = "Đang mở: " + summary
		} else {
			status = "Active: " + summary
		}
		tooltip = "AnyProfile — " + summary
	}
	if apiRunning {
		status += tx.APISuffix
	}

	return trayState{
		Active:        hasRunning || apiRunning,
		HasRunning:    hasRunning,
		StatusLabel:   status,
		OpenLabel:     tx.Open,
		SettingsLabel: tx.Settings,
		UpdatesLabel:  tx.Updates,
		StopAllLabel:  tx.StopAll,
		QuitLabel:     tx.Quit,
		Tooltip:       tooltip,
	}
}
