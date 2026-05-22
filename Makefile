.PHONY: setup pre-commit test test-go test-web format format-go format-web format-check lint build build-go build-web dev dev-api dev-web

WEB_DIR := web

setup:
	git config core.hooksPath .githooks
	chmod +x .githooks/commit-msg
	chmod +x .githooks/pre-commit

pre-commit:
	go fmt ./...
	go test ./...
	pnpm --dir $(WEB_DIR) lint-staged

test: test-go test-web

test-go:
	go test ./...

test-web:
	pnpm --dir $(WEB_DIR) test

format: format-go format-web

format-go:
	go fmt ./...

format-web:
	pnpm --dir $(WEB_DIR) format

format-check:
	pnpm --dir $(WEB_DIR) format:check

lint:
	pnpm --dir $(WEB_DIR) lint

build: build-go build-web

build-go:
	go build -o bin/patchpilot ./cmd/patchpilot

build-web:
	pnpm --dir $(WEB_DIR) build

dev:
	@set -eu; \
	$(MAKE) dev-api & \
	api_pid=$$!; \
	trap 'kill $$api_pid 2>/dev/null || true' INT TERM EXIT; \
	pnpm --dir $(WEB_DIR) dev

dev-api:
	air

dev-web:
	pnpm --dir $(WEB_DIR) dev
