GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0

.PHONY: test vet fmt-check vuln check

test:
	go test -race ./...

vet:
	go vet ./...

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "Not formatted with gofmt:"; echo "$$unformatted"; exit 1; fi

vuln:
	go run $(GOVULNCHECK) ./...

check: fmt-check vet test vuln
