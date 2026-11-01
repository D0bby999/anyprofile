// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"strings"
)

// Challenge reports a CAPTCHA or "unusual traffic" page, without solving it.
// The caller should stop and hand the window to the user.
func (b *Browser) Challenge(ctx context.Context, id string) (string, error) {
	t, err := b.tab(ctx, id)
	if err != nil {
		return "", err
	}
	var reason string
	err = b.evalJSON(ctx, t, `(() => {
		const u = location.href.toLowerCase();
		if (u.includes('/sorry/') || u.includes('captcha')) return 'url';
		for (const f of document.querySelectorAll('iframe')) {
			const s = (f.src || '').toLowerCase();
			if (s.includes('recaptcha') || s.includes('hcaptcha') || s.includes('turnstile')) return 'iframe';
		}
		const t = (document.body ? document.body.innerText : '').toLowerCase();
		if (t.includes('unusual traffic') || t.includes('not a robot')) return 'text';
		return '';
	})()`, &reason)
	if err != nil {
		return "", err
	}
	return reason, nil
}

// Interactive keeps the outline head and only lines that carry a ref, cutting
// static text for large pages. It is lossy on purpose: use snapshot for the
// full tree when the labels alone are not enough.
func Interactive(outline string) string {
	lines := strings.Split(outline, "\n")
	var b strings.Builder
	for _, line := range lines {
		if strings.HasPrefix(line, "Tab ") || strings.HasPrefix(line, "URL:") ||
			strings.HasPrefix(line, "Dialog:") || strings.Contains(line, "[ref=]") ||
			strings.Contains(line, "[ref=") {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if b.Len() == 0 {
		return outline
	}
	return b.String()
}
