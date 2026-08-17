VERSION := $(shell git describe --tags --always 2>/dev/null || echo dev)
BUILD_TIME_UTC := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.gitCommit=$(GIT_COMMIT) \
	-X main.buildTimeUTC=$(BUILD_TIME_UTC)
GOEXE := $(shell go env GOEXE)

.PHONY: build build-public build-enterprise build-sovereign test-profiles audit-sovereign tracked-patcode vet fmt lint lint-cross lint-update test desktop-test desktop-test-short desktop-test-times sdk-test sdk-test-race hooks cross clean

build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/patcode$(GOEXE) ./cmd/patcode
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/patty-plugin-example$(GOEXE) ./cmd/patty-plugin-example
	@if [ "$$(go env GOOS)" = darwin ]; then \
		/usr/bin/codesign --force --sign - bin/patcode$(GOEXE); \
		/usr/bin/codesign --force --sign - bin/patty-plugin-example$(GOEXE); \
	fi

# Deployment-tier build profiles (ADR 2026-08-16-harness-build-profiles).
# Enterprise is also the no-tags default (see internal/tier/tags_enterprise.go);
# passing the tag explicitly keeps release tooling self-documenting.
build-public:
	CGO_ENABLED=0 go build -tags profile_public -ldflags "$(LDFLAGS)" -o bin/patcode-public$(GOEXE) ./cmd/patcode

# Enterprise is the no-tags default: a bare `go build` (see
# internal/tier/tags_enterprise.go). No profile_enterprise tag exists in the
# source tree, and none may be introduced — enterprise is defined by the
# ABSENCE of profile_public/profile_sovereign.
build-enterprise:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/patcode$(GOEXE) ./cmd/patcode

build-sovereign:
	CGO_ENABLED=0 go build -tags profile_sovereign -ldflags "$(LDFLAGS)" -o bin/patcode-sovereign$(GOEXE) ./cmd/patcode

# The non-negotiable CI tax of compile-time gating: run the core suite under
# every tag set so unexercised gates cannot rot silently (ADR decision 4).
test-profiles:
	go build -tags profile_public ./... && go test -tags profile_public ./...
	go build ./... && go test ./...
	go build -tags profile_sovereign ./... && go test -tags profile_sovereign ./...
	cd desktop && go build -tags profile_public ./... && go build ./... && go build -tags profile_sovereign ./...
	cd desktop && go test -tags profile_public . && go test -tags profile_sovereign .

# ADR G2/G3/G4 consequence: sovereign binaries must provably lack external
# endpoints. Auditors grep the binary; so does CI.
#
# Pattern targets ONLY endpoint-shaped strings (full URLs), not bare hosts.
# Tightening from bare hosts to full URLs filters out false-positive
# matches from profile-agnostic host-matching logic (config.go:1640,
# cache_policy.go:61, web_search.go:31) and from prose mentions in i18n
# messages_en/ko and the embedded docs corpus (docs/embed.go — e.g.
# REASONING_PROVIDERS.md mentions https://api.deepseek.com/anthropic as
# BYOK reference prose).
#
# Tolerated in sovereign binaries, by design (ADR gate inventory):
#   - host-recognition literals (api.deepseek.com, api.xiaomimimo.com,
#     dashscope.aliyuncs.com, api.moonshot.*) — migration/feature detection
#     on user-written config entries; they dial nothing.
#   - i18n + docs-corpus prose — user-facing copy, not endpoints. This
#     includes GUIDE.md/GUIDE.ko-KR.md/CONFIG_PATHS.md example configs that
#     reference the omni vLLM-mimic endpoint and REASONING_PROVIDERS.md's
#     deepseek BYOK examples. (A profile-aware docs corpus is a G5-class
#     follow-up; the prose dials nothing.)
#   - internal mirrors — the sovereign update/plugin channel itself.
#
# Compile-time exclusion of endpoint CODE is additionally proven by the
# tier twin files (tier.AssertAllowed/AssertDisallowed at every gate
# boundary) and the per-gate materialization tests; this grep is the
# belt-and-suspenders binary check.
#
# Desktop artifact: the desktop binary is not grepped here (its CI legs run
# on runners without the desktop linking toolchain). Its sovereign endpoint
# exclusion is enforced the same way at the source level — the endpoint
# constants live only in !profile_sovereign twin files — and pinned by
# desktop/telemetry_post_sovereign_test.go and crash_post_sovereign_test.go,
# which run in the profile_sovereign desktop leg of make test-profiles.
audit-sovereign: build-sovereign
	@hits=$$(strings bin/patcode-sovereign$(GOEXE) | grep -c -E 'https://crash\.patty\.io|https://api\.github\.com|https://github\.com/pattycorp/PattyCode/releases/download|https://api\.deepseek\.com/user/balance'); \
	if [ "$$hits" -ne 0 ]; then \
		echo "audit-sovereign: FAIL - $$hits external endpoint string(s) found in sovereign binary"; \
		exit 1; \
	fi; \
	echo "audit-sovereign: OK - no external endpoints in binary"

# Refresh the checked-in macOS launcher from the canonical signed build output.
# Keeping the copy in one target prevents source fixes from being followed by a
# stale manually-copied root artifact.
tracked-patcode: build
	@if [ "$$(go env GOOS)" != darwin ] || [ -n "$(GOEXE)" ]; then \
		echo "tracked-patcode requires a macOS host" >&2; \
		exit 1; \
	fi
	cp bin/patcode patcode
	cmp -s bin/patcode patcode
	/usr/bin/codesign --verify --verbose=2 patcode

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	go run ./tools/repolint

lint-update:
	go run ./tools/repolint -update

# Linting one GOOS leaves every //go:build windows and darwin file unchecked.
lint-cross:
	@for t in "linux ." "darwin ." "windows ." "linux desktop" "windows desktop"; do \
		set -- $$t; \
		echo "== golangci-lint GOOS=$$1 ($$2)"; \
		(cd $$2 && GOOS=$$1 golangci-lint run --timeout=5m ./...) || exit 1; \
	done

test:
	go test ./...

desktop-test:
	cd desktop && go test .

desktop-test-short:
	cd desktop && go test -short .

desktop-test-times:
	cd desktop && go test -count=1 -json . | python3 ../scripts/desktop-test-times.py

sdk-test:
	cd sdk/go && go test ./...

sdk-test-race:
	cd sdk/go && go test -race ./...

hooks:
	@git config core.hooksPath .githooks
	@echo "installed: core.hooksPath -> .githooks (pre-push runs go vet)"

cross:
	@mkdir -p dist
	@for p in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "build $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o dist/patcode-$$os-$$arch$$ext ./cmd/patcode; \
	done

clean:
	rm -rf bin dist
