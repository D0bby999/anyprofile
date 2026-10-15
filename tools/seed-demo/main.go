// SPDX-License-Identifier: AGPL-3.0-or-later

// seed-demo fills a separate data directory with about sixty profiles, forty
// proxies, tags, notes, launch history and health states, so every screen of
// the app has something to show. It never touches the real data directory and
// writes nothing to the Keychain: the proxies have no password and point at
// documentation addresses (203.0.113.0/24, 198.51.100.0/24) that never connect.
//
//	go run ./tools/seed-demo -dir "$HOME/Library/Application Support/AnyProfile-Demo" [-reset]
//	open -n build/bin/AnyProfile.app --env ANYPROFILE_DATA_DIR="$HOME/Library/Application Support/AnyProfile-Demo"
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/D0bby999/anyprofile/internal/engine"
	"github.com/D0bby999/anyprofile/internal/engine/apostate"
	"github.com/D0bby999/anyprofile/internal/engine/fury"
	"github.com/D0bby999/anyprofile/internal/fingerprint"
	"github.com/D0bby999/anyprofile/internal/paths"
	"github.com/D0bby999/anyprofile/internal/service"
	"github.com/D0bby999/anyprofile/internal/store"
)

const marker = ".anyprofile-demo"

func main() {
	dir := flag.String("dir", "", "absolute path of the demo data directory")
	reset := flag.Bool("reset", false, "delete an earlier demo directory first")
	cfg := flag.String("fury-config", "build/bin/AnyProfile.app/Contents/Resources/fury-config", "fury-config helper")
	flag.Parse()
	if err := run(*dir, *cfg, *reset); err != nil {
		fmt.Fprintln(os.Stderr, "seed-demo:", err)
		os.Exit(1)
	}
}

func run(dir, cfg string, reset bool) error {
	real, err := paths.Default()
	if err != nil {
		return err
	}
	switch {
	case dir == "" || !filepath.IsAbs(dir):
		return errors.New("-dir must be an absolute path")
	case filepath.Clean(dir) == real.Root:
		return errors.New("refusing to seed the real data directory")
	}
	if _, err := os.Stat(dir); err == nil {
		if _, err := os.Stat(filepath.Join(dir, marker)); err != nil {
			return fmt.Errorf("%s exists and was not made by seed-demo; choose another path", dir)
		}
		if !reset {
			return fmt.Errorf("%s already holds demo data; add -reset to start over", dir)
		}
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	if cfg, err = filepath.Abs(cfg); err != nil {
		return err
	}
	dirs := paths.Dirs{Root: filepath.Clean(dir)}
	if err := dirs.Ensure(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, marker), []byte("made by tools/seed-demo\n"), 0o600); err != nil {
		return err
	}
	// APFS clones: the installed engines appear without a download or extra disk.
	for _, id := range []string{"apostate", "fury"} {
		if src := filepath.Join(real.Engines(), id); exists(src) {
			if out, err := exec.Command("cp", "-cR", src, dirs.Engines()+"/").CombinedOutput(); err != nil {
				return fmt.Errorf("copy engine %s: %v: %s", id, err, out)
			}
		}
	}

	st, err := store.Open(dirs.DB(), dirs.Backups())
	if err != nil {
		return err
	}
	fu := fury.New(dirs, cfg)
	svc := &service.Service{Dirs: dirs, Store: st, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Engines: map[string]engine.Engine{"fury": fu, "apostate": apostate.New(dirs)}, Preferred: "apostate",
		Locale: func(ctx context.Context, cc string) (string, []string, error) {
			loc, err := fu.LocaleFor(ctx, cc)
			return loc.UILocale, loc.Languages, err
		}}
	s := &seeder{ctx: context.Background(), svc: svc, st: st, rnd: rand.New(rand.NewPCG(7, 11)), now: time.Now().UTC(),
		perCountry: map[string]int{}}
	if err := s.seed(); err != nil {
		st.Close()
		return err
	}
	if err := st.Close(); err != nil {
		return err
	}
	if err := s.backdate(dirs.DB()); err != nil {
		return err
	}
	fmt.Printf("seeded %d profiles (%d in the trash) and %d proxies into %s\n", s.profiles, s.trashed, len(s.proxies), dir)
	return nil
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

type place struct {
	country, tz string
	lat, lon    float64
}

var places = []place{
	{"US", "America/New_York", 40.71, -74.0}, {"US", "America/Los_Angeles", 34.05, -118.24},
	{"US", "America/Chicago", 41.88, -87.63}, {"VN", "Asia/Ho_Chi_Minh", 10.82, 106.63},
	{"TW", "Asia/Taipei", 25.03, 121.56}, {"JP", "Asia/Tokyo", 35.68, 139.69},
	{"KR", "Asia/Seoul", 37.57, 126.98}, {"SG", "Asia/Singapore", 1.35, 103.82},
	{"ID", "Asia/Jakarta", -6.2, 106.85}, {"TH", "Asia/Bangkok", 13.76, 100.5},
	{"PH", "Asia/Manila", 14.6, 120.98}, {"DE", "Europe/Berlin", 52.52, 13.4},
	{"GB", "Europe/London", 51.51, -0.13}, {"FR", "Europe/Paris", 48.86, 2.35},
	{"BR", "America/Sao_Paulo", -23.55, -46.63}, {"CA", "America/Toronto", 43.65, -79.38},
	{"AU", "Australia/Sydney", -33.87, 151.21},
}

type seeder struct {
	ctx        context.Context
	svc        *service.Service
	st         *store.Store
	rnd        *rand.Rand
	now        time.Time
	proxies    []store.Proxy
	perCountry map[string]int
	profiles   int
	trashed    int
	later      []update // timestamps the store always sets to now
}

type update struct {
	query string
	args  []any
}

func (s *seeder) ago(d time.Duration) string { return s.now.Add(-d).Format(time.RFC3339) }
func days(n float64) time.Duration           { return time.Duration(n * 24 * float64(time.Hour)) }

func (s *seeder) seed() error {
	if err := s.st.SetSetting("default_engine", "apostate"); err != nil {
		return err
	}
	if err := s.seedProxies(); err != nil {
		return err
	}
	partner, err := s.svc.CreateSite(service.SiteInput{Name: "Partner portal", Domains: []string{"partner.example.com"},
		StartURL: "https://partner.example.com", Color: "#2F855A"})
	if err != nil {
		return err
	}
	type spec struct {
		name, site, country, engine, login, notes string // site: platform ids joined by "+"
		tags                                      []string
		pinned, direct, trash                     bool
		sessions                                  int
	}
	var specs []spec
	add := func(n int, f func(i int) spec) {
		for i := 1; i <= n; i++ {
			specs = append(specs, f(i))
		}
	}
	add(10, func(i int) spec {
		tags := []string{"Nuôi"}
		if i <= 3 {
			tags = []string{"Kênh chính"}
		}
		return spec{name: fmt.Sprintf("TikTok US %02d", i), site: "tiktok", country: "US", login: fmt.Sprintf("@demo_us%02d", i),
			tags: tags, pinned: i <= 3, sessions: 2 + i%5, notes: map[bool]string{true: "Kênh đang lên, đăng 2 video mỗi ngày.", false: ""}[i == 1]}
	})
	add(6, func(i int) spec {
		return spec{name: fmt.Sprintf("TikTok VN %02d", i), site: map[bool]string{true: "tiktok+youtube+gmail", false: "tiktok"}[i <= 2], country: "VN", login: fmt.Sprintf("@demo_vn%02d", i),
			tags: []string{"Nuôi"}, sessions: i % 4, notes: map[bool]string{true: "Tài khoản nuôi đợt 2, chưa bật kiếm tiền.", false: ""}[i == 2]}
	})
	add(5, func(i int) spec {
		return spec{name: fmt.Sprintf("YouTube Studio %d", i), site: "youtube", country: []string{"US", "GB", "CA", "AU", "DE"}[i-1],
			login: fmt.Sprintf("studio.%d@example.com", i), tags: []string{"Kênh chính"}, sessions: 1 + i%3}
	})
	add(4, func(i int) spec {
		return spec{name: fmt.Sprintf("Facebook Ads %d", i), site: "facebook-ads", country: "US", login: fmt.Sprintf("ads.client.a%d@example.com", i),
			tags: []string{"Khách A"}, sessions: 2, notes: map[bool]string{true: "Ngân sách 50 USD/ngày. Báo cáo thứ Hai.", false: ""}[i == 1]}
	})
	add(5, func(i int) spec {
		return spec{name: fmt.Sprintf("Seller %s", []string{"Shopee VN", "Shopee TH", "Lazada ID", "Lazada PH", "Tiki"}[i-1]),
			site: []string{"shopee", "shopee", "lazada", "lazada", "tiki"}[i-1], country: []string{"VN", "TH", "ID", "PH", "VN"}[i-1],
			login: fmt.Sprintf("shop%d@example.com", i), tags: []string{"Shop"}, sessions: 3}
	})
	add(6, func(i int) spec {
		return spec{name: fmt.Sprintf("%s %d", []string{"Instagram", "X", "Threads", "Instagram", "Pinterest", "Reddit"}[i-1], i),
			site: []string{"instagram", "x", "threads", "instagram", "pinterest", "reddit"}[i-1], country: []string{"US", "JP", "KR", "SG", "FR", "GB"}[i-1],
			sessions: i % 3}
	})
	add(4, func(i int) spec {
		return spec{name: fmt.Sprintf("%s store %d", []string{"Amazon", "eBay", "Etsy", "Amazon"}[i-1], i),
			site: []string{"amazon", "ebay", "etsy", "amazon"}[i-1], country: []string{"US", "DE", "GB", "CA"}[i-1],
			login: fmt.Sprintf("seller.b%d@example.com", i), tags: []string{"Khách B"}, sessions: 1}
	})
	add(5, func(i int) spec {
		return spec{name: fmt.Sprintf("Fury cũ %d", i), engine: "fury", site: []string{"tiktok", "youtube", "gmail", "other", "tiktok"}[i-1],
			country: []string{"US", "TW", "JP", "US", "VN"}[i-1], tags: []string{"Cũ"}, sessions: 4,
			notes: map[bool]string{true: "Tạo trước khi đổi sang Apostate. Nhân bản để chuyển engine.", false: ""}[i == 1]}
	})
	add(3, func(i int) spec {
		return spec{name: fmt.Sprintf("Partner %d", i), site: partner.ID, country: "SG", tags: []string{"Khách A"}}
	})
	add(4, func(i int) spec {
		return spec{name: fmt.Sprintf("Thử trực tiếp %d", i), site: []string{"other", "google", "tiktok", "youtube+gmail"}[i-1], direct: true,
			tags: []string{"Test"}, notes: map[bool]string{true: "Không có proxy, mở được ngay để thử.", false: ""}[i == 1]}
	})
	add(3, func(i int) spec {
		return spec{name: fmt.Sprintf("Đã bỏ %d", i), site: "tiktok", country: "US", trash: true, tags: []string{"Nuôi"}}
	})

	for _, sp := range specs {
		proxyID := ""
		if !sp.direct {
			px, err := s.nextProxy(sp.country)
			if err != nil {
				return err
			}
			proxyID = px.ID
		}
		p, err := s.svc.CreateProfile(s.ctx, service.NewProfile{Name: sp.name, Engine: sp.engine, ProxyID: proxyID,
			Sites: strings.Split(sp.site, "+"), Notes: sp.notes, Tags: sp.tags})
		if err != nil {
			return fmt.Errorf("%s: %w", sp.name, err)
		}
		if sp.pinned {
			p.Pinned = true
			if err := s.st.UpdateProfile(p); err != nil {
				return err
			}
		}
		// The login belongs to the main platform; the others get one each.
		for i, site := range p.Sites {
			login := sp.login
			if i > 0 {
				login = strings.ToLower(strings.ReplaceAll(sp.name, " ", ".")) + "." + site + "@example.com"
			}
			if login == "" || site == "other" {
				continue
			}
			if err := s.svc.SetAccountLogin(p.ID, site, login); err != nil {
				return err
			}
		}
		s.created(p, days(40+s.rnd.Float64()*60))
		if err := s.history(p, sp.sessions, nil); err != nil {
			return err
		}
		if sp.trash {
			if err := s.st.SoftDelete(p.ID); err != nil {
				return err
			}
			s.trashed++
		}
		s.profiles++
	}
	return s.statusDemos()
}

// created backdates a profile so the table's Created column varies.
func (s *seeder) created(p store.Profile, age time.Duration) {
	s.later = append(s.later, update{`UPDATE profiles SET created_at = ?, updated_at = ? WHERE id = ?`,
		[]any{s.ago(age), s.ago(age / 2), p.ID}})
}

// addProxy stores one proxy that looks checked. The hosts are documentation
// addresses, so opening a profile on one fails the way a dead proxy does.
func (s *seeder) addProxy(pl place, expires string, rotating bool, note string) (store.Proxy, error) {
	i := len(s.proxies) + 1
	host := fmt.Sprintf("203.0.113.%d", 10+i)
	if i%2 == 1 {
		host = fmt.Sprintf("198.51.100.%d", 10+i)
	}
	port, scheme := 10000+i, "http"
	if i%11 == 0 {
		scheme = "socks5"
	}
	if rotating {
		port = 823
	}
	if expires == "" && i%7 == 0 {
		expires = s.now.AddDate(0, 0, 30+i%40).Format("2006-01-02")
	}
	lat, lon := pl.lat, pl.lon
	px := store.Proxy{ID: fmt.Sprintf("demo%04d", i), Label: fmt.Sprintf("%s:%d", host, port), Scheme: scheme,
		Host: host, Port: port, ExitIP: host, Country: pl.country, Timezone: pl.tz, Lat: &lat, Lon: &lon,
		Rotating: rotating, CheckedAt: s.ago(days(1 + float64(i%9))), Note: note, ExpiresOn: expires}
	if err := s.st.CreateProxy(&px); err != nil {
		return px, err
	}
	if err := s.st.SaveProxyCheck(px); err != nil {
		return px, err
	}
	s.later = append(s.later, update{`UPDATE proxies SET checked_at = ?, created_at = ? WHERE id = ?`,
		[]any{px.CheckedAt, s.ago(days(70)), px.ID}})
	s.proxies = append(s.proxies, px)
	return px, nil
}

func placeOf(country string, n int) place {
	var in []place
	for _, pl := range places {
		if pl.country == country {
			in = append(in, pl)
		}
	}
	return in[n%len(in)]
}

// seedProxies adds the ones no profile takes: spares, one about to expire,
// one expired and one rotating port.
func (s *seeder) seedProxies() error {
	date := func(d int) string { return s.now.AddDate(0, 0, d).Format("2006-01-02") }
	for _, c := range []string{"GB", "SG", "VN"} {
		if _, err := s.addProxy(placeOf(c, 0), "", false, ""); err != nil {
			return err
		}
	}
	if _, err := s.addProxy(placeOf("US", 0), date(3), false, "Gia hạn trước thứ Sáu"); err != nil {
		return err
	}
	if _, err := s.addProxy(placeOf("VN", 0), date(-2), false, "Hết hạn, chưa gia hạn"); err != nil {
		return err
	}
	_, err := s.addProxy(placeOf("BR", 0), "", true, "Cổng xoay IP, không gán được cho profile")
	return err
}

// nextProxy gives a profile its own new proxy in the country.
func (s *seeder) nextProxy(country string) (store.Proxy, error) {
	s.perCountry[country]++
	return s.addProxy(placeOf(country, s.perCountry[country]), "", false, "")
}

func (s *seeder) proxyByNote(note string) store.Proxy {
	for _, px := range s.proxies {
		if px.Note == note {
			return px
		}
	}
	panic("no proxy noted " + note)
}

// exit describes one launch for history.
type exit struct {
	ip, country, tz, err string
	sample               func(*fingerprint.Sample)
}

// history writes n past launches, oldest first, then the fingerprint verdict
// the launches would have left. last overrides the newest launch.
func (s *seeder) history(p store.Profile, n int, last *exit) error {
	if n == 0 {
		return nil
	}
	ip, country := "", ""
	if p.ProxyID != "" {
		px, err := s.st.GetProxy(p.ProxyID)
		if err != nil {
			return err
		}
		ip, country = px.ExitIP, px.Country
	}
	sample := s.sampleFor(p)
	for k := 0; k < n; k++ {
		e := exit{ip: ip, country: country, tz: p.Timezone}
		if p.ProxyID == "" {
			e.tz = ""
		}
		if k == n-1 && last != nil {
			e = *last
		}
		x := store.Session{ProfileID: p.ID, Engine: p.Engine, EngineVersion: p.EngineVersion, ProxyID: p.ProxyID}
		if err := s.st.StartSession(&x); err != nil {
			return err
		}
		if e.ip != "" || e.err != "" {
			if err := s.st.SetSessionExit(x.ID, store.Session{ExitIP: e.ip, ExitCountry: e.country, ExitTZ: e.tz, ExitError: e.err}); err != nil {
				return err
			}
		}
		smp := sample
		if e.sample != nil {
			e.sample(&smp)
		}
		b, _ := json.Marshal(fingerprint.Checks(smp, s.expect(p)))
		if err := s.st.SetSessionChecks(x.ID, b); err != nil {
			return err
		}
		if err := s.st.EndSession(x.ID); err != nil {
			return err
		}
		start := days(float64(n-k)*2.5 + s.rnd.Float64())
		s.later = append(s.later, update{`UPDATE sessions SET started_at = ?, stopped_at = ? WHERE id = ?`,
			[]any{s.ago(start), s.ago(start - time.Duration(10+s.rnd.IntN(80))*time.Minute), x.ID}})
	}
	base, _ := json.Marshal(sample)
	if err := s.st.SaveFingerprint(p.ID, string(base), string(base), map[bool]string{true: "baseline", false: "match"}[n == 1], nil); err != nil {
		return err
	}
	s.later = append(s.later, update{`UPDATE profiles SET last_open_at = ?, fp_checked_at = ? WHERE id = ?`,
		[]any{s.ago(days(2.5)), s.ago(days(2.5)), p.ID}})
	return nil
}

func (s *seeder) expect(p store.Profile) fingerprint.Expect {
	e := fingerprint.Expect{Engine: p.Engine, Timezone: p.Timezone, Languages: p.Languages, ChromiumMajor: 155,
		Proxied: p.ProxyID != "", PrimaryLangOnly: p.Engine == "apostate"}
	if p.Engine == "fury" {
		e.ChromiumMajor = 145
	} else {
		e.HWC = p.EngineMeta.HWC
	}
	return e
}

// sampleFor is what a healthy launch of the profile measures.
func (s *seeder) sampleFor(p store.Profile) fingerprint.Sample {
	exp := s.expect(p)
	gpus := []string{"Apple M3 Pro", "Apple M2", "Apple M4 Pro", "Apple M1", "Apple M3 Max"}
	screens := []string{"1512x982", "1440x900", "1728x1117", "2560x1440", "1470x956"}
	g := int(p.SeedHex[0]) % len(gpus)
	langs := p.Languages
	if exp.PrimaryLangOnly {
		langs, _, _ = strings.Cut(langs, ",")
	}
	hwc := exp.HWC
	if hwc == 0 {
		hwc = 8
	}
	return fingerprint.Sample{Canvas: p.SeedHex, WebGLVendor: "Google Inc. (Apple)",
		WebGLRenderer: "ANGLE (Apple, ANGLE Metal Renderer: " + gpus[g] + ", Unspecified Version)",
		UA:            fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36", exp.ChromiumMajor),
		Platform:      "MacIntel", HWC: hwc, Mem: 8, Screen: screens[g], TZ: p.Timezone, Langs: langs, Secure: true,
		ChromeH: -1, Geo: "denied"}
}

// statusDemos are profiles that each show one health state. Opening them
// replaces the seeded state with a real measurement.
func (s *seeder) statusDemos() error {
	tags := []string{"Trạng thái mẫu"}
	mk := func(name, country string, proxy *store.Proxy) (store.Profile, error) {
		id := ""
		if proxy != nil {
			id = proxy.ID
		} else if country != "" {
			px, err := s.nextProxy(country)
			if err != nil {
				return store.Profile{}, err
			}
			id = px.ID
		}
		p, err := s.svc.CreateProfile(s.ctx, service.NewProfile{Name: name, ProxyID: id, Sites: []string{"tiktok"}, Tags: tags})
		if err == nil {
			s.created(p, days(20))
			s.profiles++
		}
		return p, err
	}
	steps := []struct {
		name, country string
		do            func(p store.Profile) error
	}{
		{"Mẫu: fingerprint đổi", "US", func(p store.Profile) error {
			if err := s.history(p, 3, nil); err != nil {
				return err
			}
			return s.st.SaveFingerprint(p.ID, "", "", "drift", []string{"canvas", "webglRenderer"})
		}},
		{"Mẫu: đổi quốc gia", "US", func(p store.Profile) error {
			return s.history(p, 4, &exit{ip: "203.0.113.250", country: "CA", tz: p.Timezone})
		}},
		{"Mẫu: đổi IP", "JP", func(p store.Profile) error {
			return s.history(p, 3, &exit{ip: "203.0.113.251", country: "JP", tz: p.Timezone})
		}},
		{"Mẫu: lệch múi giờ", "US", func(p store.Profile) error {
			px, _ := s.st.GetProxy(p.ProxyID)
			return s.history(p, 2, &exit{ip: px.ExitIP, country: "US", tz: "America/Denver"})
		}},
		{"Mẫu: không đo được IP", "DE", func(p store.Profile) error {
			return s.history(p, 2, &exit{err: "proxy did not answer within 10 s"})
		}},
		{"Mẫu: WebRTC lộ IP", "TW", func(p store.Profile) error {
			px, _ := s.st.GetProxy(p.ProxyID)
			return s.history(p, 2, &exit{ip: px.ExitIP, country: px.Country, tz: p.Timezone,
				sample: func(x *fingerprint.Sample) { x.ICE = []string{"192.0.2.44"} }})
		}},
		{"Mẫu: ngôn ngữ lệch", "KR", func(p store.Profile) error {
			px, _ := s.st.GetProxy(p.ProxyID)
			return s.history(p, 2, &exit{ip: px.ExitIP, country: px.Country, tz: p.Timezone,
				sample: func(x *fingerprint.Sample) { x.Langs = "en-US" }})
		}},
		{"Mẫu: chưa đo fingerprint", "TH", func(p store.Profile) error {
			if err := s.history(p, 1, nil); err != nil {
				return err
			}
			return s.st.SaveFingerprint(p.ID, "", "", "unchecked", nil)
		}},
	}
	for _, st := range steps {
		p, err := mk(st.name, st.country, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", st.name, err)
		}
		if err := st.do(p); err != nil {
			return fmt.Errorf("%s: %w", st.name, err)
		}
	}
	for name, note := range map[string]string{"Mẫu: proxy sắp hết hạn": "Gia hạn trước thứ Sáu", "Mẫu: proxy đã hết hạn": "Hết hạn, chưa gia hạn"} {
		px := s.proxyByNote(note)
		p, err := mk(name, "", &px)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := s.history(p, 2, nil); err != nil {
			return err
		}
	}
	return nil
}

// backdate sets the times the store always stamps with now.
func (s *seeder) backdate(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, u := range s.later {
		if _, err := db.Exec(u.query, u.args...); err != nil {
			return err
		}
	}
	return nil
}
