// SPDX-License-Identifier: AGPL-3.0-or-later

package control

import "testing"

func TestParseNotifications(t *testing.T) {
	dump := `NotificationRecord(0x1: pkg=com.test user=0)
  android.title=String (Hello)
  android.text=String (World)
NotificationRecord(0x2: pkg=com.other user=0)
  android.title=Plain
NotificationRecord(0x3: pkg=com.empty user=0)
`
	ns := parseNotifications(dump, 10)
	if len(ns) != 2 {
		t.Fatalf("got %d notifications: %+v", len(ns), ns)
	}
	if ns[0].Package != "com.test" || ns[0].Title != "Hello" || ns[0].Text != "World" {
		t.Fatalf("first = %+v", ns[0])
	}
	if ns[1].Package != "com.other" || ns[1].Title != "Plain" {
		t.Fatalf("second = %+v", ns[1])
	}
	if len(parseNotifications(dump, 1)) != 1 {
		t.Fatal("limit not applied")
	}
}
