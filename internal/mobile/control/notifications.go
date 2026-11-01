// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import (
	"context"
	"regexp"
	"strings"
)

// Notification is one active notification, as dumpsys reports it.
type Notification struct {
	Package string `json:"package"`
	Title   string `json:"title"`
	Text    string `json:"text"`
}

var (
	notifPkg   = regexp.MustCompile(`pkg=([^\s]+)`)
	notifTitle = regexp.MustCompile(`android\.title=(?:String \()?(.*?)\)?$`)
	notifText  = regexp.MustCompile(`android\.text=(?:String \()?(.*?)\)?$`)
)

// Notifications lists active notifications (up to limit) with package, title
// and text. The dump is untrusted text from apps; it is returned as data.
func (d *Device) Notifications(ctx context.Context, limit int) ([]Notification, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	out, err := d.shell(ctx, "dumpsys notification --noredact 2>/dev/null")
	if err != nil {
		return nil, err
	}
	return parseNotifications(out, limit), nil
}

func parseNotifications(dump string, limit int) []Notification {
	var out []Notification
	for _, block := range strings.Split(dump, "NotificationRecord(")[1:] {
		if len(out) >= limit {
			break
		}
		m := notifPkg.FindStringSubmatch(block)
		if m == nil {
			continue
		}
		n := Notification{Package: m[1]}
		for _, line := range strings.Split(block, "\n") {
			if t := notifTitle.FindStringSubmatch(strings.TrimSpace(line)); t != nil {
				n.Title = strings.TrimSpace(t[1])
			}
			if t := notifText.FindStringSubmatch(strings.TrimSpace(line)); t != nil {
				n.Text = strings.TrimSpace(t[1])
			}
		}
		if n.Title == "" && n.Text == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}
