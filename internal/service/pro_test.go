// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/D0bby999/anyprofile/internal/pro"
)

// fakePro is the license server in a few lines: one good key, packs by version.
type fakePro struct {
	mu       sync.Mutex
	priv     ed25519.PrivateKey
	key      string
	state    string
	revoked  bool
	packs    map[string][]byte
	latest   string
	claims   map[string]string
	removed  []string
	asks     int
	runtimes []pro.Runtime
}

func newFakePro(t *testing.T, s *Service) *fakePro {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	f := &fakePro{priv: priv, key: "APRO-AAAAA-BBBBB-CCCCC-DDDDD", state: "active", packs: map[string][]byte{}, claims: map[string]string{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	s.Pro = pro.Client{Base: srv.URL, HTTP: srv.Client(), Key: pub}
	return f
}

func (f *fakePro) publish(t *testing.T, version string) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range map[string]string{"VERSION": version + "\n", "recipes/pro-task.md": "---\nname: pro-task\ndescription: d\n---\nbody " + version + "\n", "scripts/run.mjs": "x"} {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.mu.Lock()
	f.packs[version], f.latest = buf.Bytes(), version
	f.mu.Unlock()
}

func (f *fakePro) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fail := func(status int, code string) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]string{"error": code})
	}
	var in pro.Ask
	var raw map[string]string
	if r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &in)
		json.Unmarshal(body, &raw)
	}
	switch {
	case r.URL.Path == "/v1/claim":
		k, ok := f.claims[r.URL.Query().Get("ref")]
		if !ok {
			fail(404, "not_yet")
			return
		}
		delete(f.claims, r.URL.Query().Get("ref"))
		json.NewEncoder(w).Encode(map[string]string{"key": k})
	case r.URL.Path == "/v1/pack":
		f.asks++
		if in.Key != f.key {
			fail(401, "bad_key")
			return
		}
		if f.revoked {
			fail(401, "revoked")
			return
		}
		out := pro.Answer{State: f.state, PaidThrough: 1_900_000_000, Runtimes: f.runtimes}
		if b := f.packs[f.latest]; b != nil && f.latest != in.Have {
			sum := sha256.Sum256(b)
			h := hex.EncodeToString(sum[:])
			out.Release = &pro.Release{Version: f.latest, SHA256: h, Size: int64(len(b)), Notes: "notes " + f.latest,
				Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, pro.SignedMessage(f.latest, h))),
				URL: "http://" + r.Host + "/v1/download/" + f.latest}
		}
		json.NewEncoder(w).Encode(out)
	case strings.HasPrefix(r.URL.Path, "/v1/download/"):
		w.Write(f.packs[strings.TrimPrefix(r.URL.Path, "/v1/download/")])
	case r.URL.Path == "/v1/device/remove":
		f.removed = append(f.removed, raw["install_id"])
		w.WriteHeader(204)
	default:
		fail(404, "not_found")
	}
}

func TestProActivateInstallsAndUpdates(t *testing.T) {
	s, sec := newService(t)
	f := newFakePro(t, s)
	f.publish(t, "1.2.0")
	ctx := context.Background()

	if _, err := s.ProActivate(ctx, "APRO-WRONG-WRONG-WRONG-WRONG"); err == nil {
		t.Fatal("a wrong key was taken")
	}
	if _, err := sec.Get(proVault, proAccount); err == nil {
		t.Fatal("a refused key was kept")
	}
	st, err := s.ProActivate(ctx, " apro-aaaaa-bbbbb-ccccc-ddddd ")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Active || st.State != "active" || st.Version != "1.2.0" || st.Key != "APRO-•••••-•••••-•••••-DDDDD" || st.Notes != "notes 1.2.0" {
		t.Fatalf("status %+v", st)
	}
	if _, err := os.Stat(filepath.Join(pro.Recipes(s.Dirs.Pro()), "pro-task.md")); err != nil {
		t.Fatal(err)
	}

	// Within a day nothing is asked; forced, the new pack comes.
	f.publish(t, "1.3.0")
	asks := f.asks
	if st, _ = s.ProCheck(ctx, false); st.Version != "1.2.0" || f.asks != asks {
		t.Fatalf("checked again within a day: %+v", st)
	}
	if st, err = s.ProCheck(ctx, true); err != nil || st.Version != "1.3.0" {
		t.Fatalf("forced check: %+v %v", st, err)
	}
}

func TestProClaimAfterPaying(t *testing.T) {
	s, _ := newService(t)
	f := newFakePro(t, s)
	f.publish(t, "1.2.0")
	u, ref, err := s.ProBuyURL("year")
	if err != nil || !strings.Contains(u, "plan=year") || !strings.Contains(u, "ref="+ref) {
		t.Fatalf("buy url %q %v", u, err)
	}
	if done, err := s.ProClaim(context.Background(), ref); done || err != nil {
		t.Fatalf("claimed before paying: %v %v", done, err)
	}
	f.claims[ref] = f.key
	if done, err := s.ProClaim(context.Background(), ref); !done || err != nil {
		t.Fatalf("claim after paying: %v %v", done, err)
	}
	if st := s.ProStatus(); !st.Active || st.Version != "1.2.0" {
		t.Fatalf("status %+v", st)
	}
	if _, _, err := s.ProBuyURL("lifetime"); err == nil {
		t.Fatal("an unknown plan")
	}
}

func TestProRevokedAndDeactivated(t *testing.T) {
	s, _ := newService(t)
	f := newFakePro(t, s)
	f.publish(t, "1.2.0")
	ctx := context.Background()
	if _, err := s.ProActivate(ctx, f.key); err != nil {
		t.Fatal(err)
	}
	f.revoked = true
	st, err := s.ProCheck(ctx, true)
	if err == nil || st.State != "revoked" || st.Version != "" {
		t.Fatalf("revoked: %+v %v", st, err)
	}

	f.revoked = false
	if _, err := s.ProActivate(ctx, f.key); err != nil {
		t.Fatal(err)
	}
	id, _ := s.Store.GetSetting("pro_install_id")
	if err := s.ProDeactivate(ctx); err != nil {
		t.Fatal(err)
	}
	if st := s.ProStatus(); st.Active || st.Version != "" || st.State != "" {
		t.Fatalf("after removing: %+v", st)
	}
	if len(f.removed) != 1 || f.removed[0] != id {
		t.Fatalf("this computer's place was not freed: %v", f.removed)
	}
}

func TestProWithoutKeyAsksNothing(t *testing.T) {
	s, _ := newService(t)
	f := newFakePro(t, s)
	if st, err := s.ProCheck(context.Background(), true); err != nil || st.Active {
		t.Fatalf("%+v %v", st, err)
	}
	if f.asks != 0 {
		t.Fatal("the server was asked without a key")
	}
}

func TestProAccountPortalAndEmailPrefill(t *testing.T) {
	s, _ := newService(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/account/portal", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer acc_tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"url":"https://billing.stripe.test/cus_acc"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	s.Pro = pro.Client{Base: srv.URL, HTTP: srv.Client()}

	// Signed-out buy URL has no email query param.
	u0, _, err := s.ProBuyURL("year")
	if err != nil || strings.Contains(u0, "email=") {
		t.Fatalf("signed-out buy url = %q %v", u0, err)
	}

	// Signed-in account prefills email on BuyURL / BuyGiftURL and opens portal without a local key.
	s.saveAccountEmail("buyer@example.com")
	if err := s.saveAccountTokens(pro.AccountTokens{AccessToken: "acc_tok", RefreshToken: "ref_tok"}); err != nil {
		t.Fatal(err)
	}
	u1, _, err := s.ProBuyURL("month")
	if err != nil || !strings.Contains(u1, "email=buyer%40example.com") {
		t.Fatalf("signed-in buy url = %q %v", u1, err)
	}
	if ug := s.ProBuyGiftURL(); !strings.Contains(ug, "email=buyer%40example.com") {
		t.Fatalf("signed-in gift url = %q", ug)
	}
	portal, err := s.ProPortalURL(context.Background())
	if err != nil || portal != "https://billing.stripe.test/cus_acc" {
		t.Fatalf("account portal = %q %v", portal, err)
	}
}
