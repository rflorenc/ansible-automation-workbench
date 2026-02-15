.PHONY: build dev frontend backend clean snapshot release

export PATH := $(HOME)/.local/node/bin:$(HOME)/.local/go/bin:$(PATH)
GO := $(shell which go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
NPM := $(shell which npm 2>/dev/null || echo $(HOME)/.local/node/bin/npm)

VERSION ?= dev
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build: frontend backend

frontend:
	cd web && $(NPM) ci && $(NPM) run build

backend:
	CGO_ENABLED=0 $(GO) build -ldflags "$(LDFLAGS)" -o autoworkbench ./cmd/workbench/

build-run: frontend backend
	./autoworkbench --config config.yaml

dev:
	@echo "Run in two terminals:"
	@echo "  Terminal 1: cd web && npm run dev"
	@echo "  Terminal 2: $(GO) run ./cmd/workbench/ --dev"

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

clean:
	rm -rf web/dist web/node_modules autoworkbench dist/
