# SPDX-License-Identifier: AGPL-3.0-or-later
WAILS       ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
GOVULNCHECK ?= $(shell command -v govulncheck 2>/dev/null || echo $(HOME)/go/bin/govulncheck)
FURY_CONFIG := tools/fury-config/target/release/fury-config
APP         := build/bin/AnyProfile.app
# The app's version, from wails.json; the in-app updater compares against it.
VERSION     ?= $(shell sed -n 's/.*"productVersion": *"\([^"]*\)".*/\1/p' wails.json)
LDFLAGS     := -X github.com/D0bby999/anyprofile/internal/update.Current=$(VERSION)

.PHONY: dev build build-windows fury-config test test-integration lint

fury-config:
	cd tools/fury-config && cargo build --release --locked

dev: fury-config
	$(WAILS) dev

# Signing identity: "AnyProfile Dev" from scripts/make-dev-cert.sh when it
# exists; ad hoc otherwise. Without an Apple Team ID, macOS still keys
# Keychain "Always Allow" to each build's cdhash, so a rebuild asks again,
# once, at launch (secrets.Vault: one Keychain item for every secret).
SIGN_ID ?= $(shell security find-identity -v -p codesigning 2>/dev/null | grep -q '"AnyProfile Dev"' && echo 'AnyProfile Dev' || echo -)
# A release signed with a Developer ID adds the hardened runtime and a secure
# timestamp, which notarization requires (release.yml sets both).
SIGN_FLAGS ?=

# wails build signs the bundle ad hoc; adding the helper breaks that seal,
# so the bundle is re-signed after the copy.
build: fury-config
	$(WAILS) build -clean -platform darwin/arm64 -ldflags "$(LDFLAGS)"
	cp $(FURY_CONFIG) $(APP)/Contents/Resources/fury-config
	codesign --force --deep $(SIGN_FLAGS) --sign "$(SIGN_ID)" $(APP)
	codesign --verify --deep --strict $(APP)

# The disk image people download (open it, drag AnyProfile to Applications).
dmg: build
	scripts/make-dmg.sh $(APP) build/bin/AnyProfile-macos-arm64.dmg

# Windows x64 app and installer. Runs on Windows (Git Bash): fury-config needs
# the MSVC Rust toolchain and the installer NSIS. CI runs the same steps in
# .github/workflows/windows.yml. Output: build/bin/AnyProfile-amd64-installer.exe.
build-windows: fury-config
	mkdir -p build/bin
	cp $(FURY_CONFIG).exe build/bin/fury-config.exe
	$(WAILS) build -platform windows/amd64 -nsis -ldflags "$(LDFLAGS)"

test: fury-config
	go test ./internal/... -count=1
	cd frontend && npm run typecheck && npm test

# Needs ANYPROFILE_TEST_FURY_APP=<path to a verified Fury.app>.
test-integration: fury-config
	go test -tags integration ./internal/launcher/ ./internal/service/ -count=1 -v

lint:
	./scripts/check-spdx.sh
	gofmt -l . | (! grep .) || (echo "gofmt needed" >&2; exit 1)
	go vet ./...
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet -tags "integration network keychain" ./...
	$(GOVULNCHECK) ./...
	cd frontend && npm audit --omit=dev
	cd tools/fury-config && cargo clippy --release --locked -- -D warnings
