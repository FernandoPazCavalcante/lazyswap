GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

# Pinned so a local `make gate` and CI agree. Bump both here and in
# .github/workflows/{ci,nightly}.yml together.
GOLANGCI_VERSION ?= v2.15.0
GREMLINS_VERSION ?= v0.5.0

GOLANGCI ?= $(GOBIN)/golangci-lint
GREMLINS ?= $(GOBIN)/gremlins

.PHONY: build setup tools-check lint test cover e2e mutate gate

build:
	go build ./...

# One-time setup for a fresh clone: installs the pinned lint and mutation
# tools into $(GOBIN) — the same path the targets below look in.
setup:
	@echo "installing golangci-lint $(GOLANGCI_VERSION) -> $(GOBIN)"
	@GOBIN=$(GOBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)
	@echo "installing gremlins $(GREMLINS_VERSION) -> $(GOBIN)"
	@GOBIN=$(GOBIN) go install github.com/go-gremlins/gremlins/cmd/gremlins@$(GREMLINS_VERSION)
	@echo "done — run 'make gate'"

tools-check:
	@test -x "$(GOLANGCI)" || { echo "golangci-lint not found at $(GOLANGCI) — run 'make setup'"; exit 1; }

lint: tools-check
	$(GOLANGCI) run ./...

test:
	LAZYSWAP_TEST=1 go test -race ./...

cover:
	bash scripts/coverage-gate.sh

e2e:
	LAZYSWAP_TEST=1 go test -tags e2e -race -count=1 ./e2e/...

mutate:
	@test -x "$(GREMLINS)" || { echo "gremlins not found at $(GREMLINS) — run 'make setup'"; exit 1; }
	LAZYSWAP_TEST=1 $(GREMLINS) unleash --threshold-efficacy 50

# Local mirror of the CI PR gate.
gate: lint cover
