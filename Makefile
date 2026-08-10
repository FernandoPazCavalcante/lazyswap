GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif
GOLANGCI ?= $(GOBIN)/golangci-lint
GREMLINS ?= $(GOBIN)/gremlins

.PHONY: build lint test cover e2e mutate gate

build:
	go build ./...

lint:
	$(GOLANGCI) run ./...

test:
	LAZYSWAP_TEST=1 go test -race ./...

cover:
	bash scripts/coverage-gate.sh

e2e:
	LAZYSWAP_TEST=1 go test -tags e2e -race -count=1 ./e2e/...

mutate:
	LAZYSWAP_TEST=1 $(GREMLINS) unleash --threshold-efficacy 50

# Local mirror of the CI PR gate.
gate: lint cover
