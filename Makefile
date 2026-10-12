# SPDX-License-Identifier: AGPL-3.0-or-later
WAILS       ?= $(shell command -v wails 2>/dev/null || echo $(HOME)/go/bin/wails)
GOVULNCHECK ?= $(shell command -v govulncheck 2>/dev/null || echo $(HOME)/go/bin/govulncheck)
FURY_CONFIG := tools/fury-config/target/release/fury-config
APP         := build/bin/AnyProfile.app

.PHONY: dev build fury-config test test-integration lint

fury-config:
	cd tools/fury-config && cargo build --release --locked

dev: fury-config
	$(WAILS) dev

# wails build signs the bundle ad hoc; adding the helper breaks that seal,
# so the bundle is re-signed after the copy.
build: fury-config
	$(WAILS) build -clean -platform darwin/arm64
	cp $(FURY_CONFIG) $(APP)/Contents/Resources/fury-config
	codesign --force --deep --sign - $(APP)
	codesign --verify --deep --strict $(APP)

test: fury-config
	go test ./internal/... -count=1
	cd frontend && npm run typecheck && npm test

# Needs ANYPROFILE_TEST_FURY_APP=<path to a verified Fury.app>.
test-integration: fury-config
	go test -tags integration ./internal/launcher/ -count=1 -v

lint:
	./scripts/check-spdx.sh
	gofmt -l . | (! grep .) || (echo "gofmt needed" >&2; exit 1)
	go vet ./...
	$(GOVULNCHECK) ./...
	cd frontend && npm audit --omit=dev
	cd tools/fury-config && cargo clippy --release --locked -- -D warnings
