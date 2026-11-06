GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0
WAILS_MODULE := github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
WAILS := go run $(WAILS_MODULE)
# The Wails tasks call wails3 by name: the pinned one is installed here, ahead of anything on PATH.
WAILS_BIN := $(CURDIR)/.task/bin

# Container tests (Docker needed): make test-integration; make record refreshes the WP-CLI outputs.
.PHONY: test-integration record test vet fmt-check vuln go-check frontend-install frontend-browsers frontend-lint frontend-test frontend-build frontend-e2e bindings bindings-check library app wails release check

# The module shared with add-ons (./addon) is a module of its own: ./... does not reach it.
test:
	go test -race ./...
	cd addon && go test -race ./...

# ./... skips testdata: the program that prepares the end-to-end test is named as well.
GO_TESTDATA := ./cmd/vigie/testdata/seed

vet:
	go vet ./... $(GO_TESTDATA)
	go vet -tags integration ./internal/integration
	go vet -tags server ./cmd/vigie
	cd addon && go vet ./...

# Only the module's packages: gofmt alone would also walk frontend/node_modules.
fmt-check:
	@unformatted="$$(gofmt -l $$(go list -f '{{.Dir}}' ./... $(GO_TESTDATA)) addon)"; \
	if [ -n "$$unformatted" ]; then echo "Not formatted with gofmt:"; echo "$$unformatted"; exit 1; fi

vuln:
	go run $(GOVULNCHECK) ./...

go-check: fmt-check vet test vuln

# The bench builds vigie for vigie mcp (ADR-0043), which embeds frontend/dist: without a built
# interface (the CI job has no Node), a page in its place for the bench's build, removed after.
test-integration:
	@placeholder=; if [ ! -e frontend/dist/index.html ]; then mkdir -p frontend/dist && \
		printf '<!doctype html><title>Vigie</title>\n' > frontend/dist/index.html && placeholder=1; fi; \
	go test -tags integration -count=1 -timeout 20m ./internal/integration; status=$$?; \
	if [ -n "$$placeholder" ]; then rm -rf frontend/dist; fi; exit $$status

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

# The Library's descriptions, from the add-ons installed here (each one listed must be).
library:
	go generate ./internal/addons

# Vigie.app (macOS, signed ad hoc) or the Linux packages, with the Wails tasks; VERSION=x.y.z writes
# a release's version in them, "dev" otherwise.
app: wails
	PATH="$(WAILS_BIN):$$PATH" "$(WAILS_BIN)/wails3" package $(if $(VERSION),VERSION=$(VERSION))

wails:
	GOBIN="$(WAILS_BIN)" go install $(WAILS_MODULE)

# A release, on the owner's Mac once the tag vx.y.z is pushed (docs/releasing.md):
# make release VERSION=x.y.z KEY=<private key>
release: wails
	WAILS_BIN="$(WAILS_BIN)" VERSION="$(VERSION)" KEY="$(KEY)" build/release.sh

# The interface is built first: the Go binary embeds frontend/dist.
check: frontend-build go-check bindings-check frontend-lint frontend-test frontend-e2e
