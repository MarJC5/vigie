GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0
WAILS := go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0

.PHONY: test vet fmt-check vuln go-check frontend-install frontend-lint frontend-test frontend-build bindings bindings-check app check

test:
	go test -race ./...

vet:
	go vet ./...

# Only the module's packages: gofmt alone would also walk frontend/node_modules.
fmt-check:
	@unformatted="$$(gofmt -l $$(go list -f '{{.Dir}}' ./...))"; \
	if [ -n "$$unformatted" ]; then echo "Not formatted with gofmt:"; echo "$$unformatted"; exit 1; fi

vuln:
	go run $(GOVULNCHECK) ./...

go-check: fmt-check vet test vuln

frontend-install:
	cd frontend && npm ci --no-audit --no-fund

frontend-lint:
	cd frontend && npm run lint && npm run typecheck

frontend-test:
	cd frontend && npm test

frontend-build:
	cd frontend && npm run build

# TypeScript bindings generated from the Go services, committed; CI fails when they are stale.
bindings:
	$(WAILS) generate bindings -ts -i -clean=true ./cmd/vigie

bindings-check:
	@tmp="$$(mktemp -d)"; \
	$(WAILS) generate bindings -ts -i -silent -d "$$tmp" ./cmd/vigie && \
	diff -r "$$tmp" frontend/bindings || { echo "Bindings are stale: run make bindings"; exit 1; }

# Vigie.app (macOS, signed ad hoc) or the Linux build, with the Wails tasks.
app:
	$(WAILS) package

# The interface is built first: the Go binary embeds frontend/dist.
check: frontend-build go-check bindings-check frontend-lint frontend-test
