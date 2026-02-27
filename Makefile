.PHONY: build dev frontend backend clean snapshot release container-build container-run container-push

export PATH := $(HOME)/.local/node/bin:$(HOME)/.local/go/bin:$(PATH)
GO := $(shell which go 2>/dev/null || echo $(HOME)/.local/go/bin/go)
NPM := $(shell which npm 2>/dev/null || echo $(HOME)/.local/node/bin/npm)
CONTAINER_ENGINE ?= $(shell command -v podman 2>/dev/null || command -v docker 2>/dev/null)
IMAGE ?= quay.io/rlourencc/ansible-automation-workbench

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

build-run-dev: frontend backend
	./autoworkbench --config .config.yaml

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

container-build:
	$(CONTAINER_ENGINE) build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg DATE=$(DATE) .

container-run:
	$(CONTAINER_ENGINE) run --rm --network host -v $(PWD)/config.yaml:/config/config.yaml:ro $(IMAGE):$(VERSION)

container-push:
	$(CONTAINER_ENGINE) push $(IMAGE):$(VERSION)
	$(CONTAINER_ENGINE) push $(IMAGE):latest
