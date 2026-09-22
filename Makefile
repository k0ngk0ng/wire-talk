export GOCACHE := $(CURDIR)/.cache/go-build
export GOMODCACHE := $(CURDIR)/.cache/go-mod
export GOTMPDIR := $(CURDIR)/.cache/tmp
export TMPDIR := $(CURDIR)/.cache/tmp

.PHONY: prepare test build check
prepare:
	mkdir -p .cache/go-build .cache/go-mod .cache/tmp bin
build: prepare
	go build -trimpath -o bin/wirectl ./cmd/wirectl
	go build -trimpath -o bin/wirectl-talk ./cmd/wirectl-talk
test: prepare
	go test -race ./...
check: test
	go vet ./...
