BINARY  := adaptive-cache
PKG     := ./...
CONFIG  ?= configs/default.yaml
DURATION ?= 5s

.PHONY: build test test-race lint lint-arch run clean

build:
	go build -o $(BINARY) ./cmd/adaptive-cache

test:
	go test $(PKG)

test-race:
	go test -race $(PKG)

# `lint` is deliberately limited to the tooling that ships with Go, so CI needs
# no extra installs.
lint:
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then \
		echo "gofmt required for:"; echo "$$out"; exit 1; fi
	go vet $(PKG)

lint-arch:
	./scripts/lint-arch.sh

run: build
	./$(BINARY) --config $(CONFIG) --duration $(DURATION)

clean:
	rm -f $(BINARY)
	go clean -testcache
