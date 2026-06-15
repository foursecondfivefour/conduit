.PHONY: build release test vet clean lint bench security

VERSION ?= $(shell powershell -Command "& .\.github\scripts\read-version.ps1")
LDFLAGS := -s -w -H=windowsgui -buildid= -X github.com/foursecondfivefour/conduit/internal/config.Version=$(VERSION)

build:
	go build -trimpath -ldflags="$(LDFLAGS)" -tags production -o build/conduit.exe .

test:
	go test ./... -count=1 -timeout=5m

vet:
	go vet ./...

lint:
	golangci-lint run --timeout=5m ./...

race:
	go test -race ./... -count=1 -timeout=10m

security:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

release:
	go build -trimpath -ldflags="$(LDFLAGS)" -tags production -o build/conduit.exe .
	go build -trimpath -ldflags="-s -w -H=windowsgui -buildid=" -tags production -o build/conduit-updater.exe ./cmd/conduit-updater

clean:
	rm -rf build/

bench:
	go test -bench=. -benchmem ./...
