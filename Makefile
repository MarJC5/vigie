GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0

.PHONY: test vet fmt-check vuln go-check frontend-install frontend-lint frontend-test frontend-build check

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

check: go-check frontend-lint frontend-test frontend-build
