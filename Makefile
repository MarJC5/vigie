GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0
WAILS_MODULE := github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
WAILS := go run $(WAILS_MODULE)
# The Wails tasks call wails3 by name: the pinned one is installed here, ahead of anything on PATH.
WAILS_BIN := $(CURDIR)/.task/bin

# Container tests (Docker needed): make test-integration; make record refreshes the WP-CLI outputs.
.PHONY: test-integration record test vet fmt-check vuln go-check frontend-install frontend-browsers frontend-lint frontend-test frontend-build frontend-e2e bindings bindings-check app check

test:
	go test -race ./...

# ./... skips testdata: the program that prepares the end-to-end test is named as well.
GO_TESTDATA := ./cmd/vigie/testdata/seed

vet:
	go vet ./... $(GO_TESTDATA)
	go vet -tags integration ./internal/integration

# Only the module's packages: gofmt alone would also walk frontend/node_modules.
fmt-check:
	@unformatted="$$(gofmt -l $$(go list -f '{{.Dir}}' ./... $(GO_TESTDATA)))"; \
	if [ -n "$$unformatted" ]; then echo "Not formatted with gofmt:"; echo "$$unformatted"; exit 1; fi

vuln:
	go run $(GOVULNCHECK) ./...

go-check: fmt-check vet test vuln

test-integration:
	go test -tags integration -count=1 -timeout 20m ./internal/integration

record:
	go test -tags integration -count=1 -timeout 20m -run TestRecord ./internal/integration -record

frontend-install:
	cd frontend && npm ci --no-audit --no-fund

# Chromium and WebKit run the component stories; --with-deps adds their system libraries on Linux.
frontend-browsers:
	cd frontend && npx playwright install $(if $(CI),--with-deps,) chromium webkit

frontend-lint:
	cd frontend && npm run lint && npm run typecheck

frontend-test:
	cd frontend && npm test

frontend-build:
	cd frontend && npm run build

# The window end to end: the binary built in server mode, in Chromium and WebKit (needs frontend-build).
frontend-e2e:
	cd frontend && npx playwright test

# TypeScript bindings generated from the Go services, committed; CI fails when they are stale.
bindings:
	$(WAILS) generate bindings -ts -i -clean=true ./cmd/vigie

bindings-check:
	@tmp="$$(mktemp -d)"; \
	$(WAILS) generate bindings -ts -i -silent -d "$$tmp" ./cmd/vigie && \
	diff -r "$$tmp" frontend/bindings || { echo "Bindings are stale: run make bindings"; exit 1; }

# Vigie.app (macOS, signed ad hoc) or the Linux build, with the Wails tasks.
app:
	GOBIN="$(WAILS_BIN)" go install $(WAILS_MODULE)
	PATH="$(WAILS_BIN):$$PATH" "$(WAILS_BIN)/wails3" package

# The interface is built first: the Go binary embeds frontend/dist.
check: frontend-build go-check bindings-check frontend-lint frontend-test frontend-e2e
