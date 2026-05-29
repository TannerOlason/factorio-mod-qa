.PHONY: test test-go build doctor validate-fixture

GO ?= go
GOCACHE ?= /tmp/go-build-cache
FACTORIO_BIN ?= /home/user/.factorio

test: test-go

test-go:
	GOCACHE=$(GOCACHE) $(GO) test ./...

build:
	GOCACHE=$(GOCACHE) $(GO) build -buildvcs=false -o /tmp/fmqa ./cmd/fmqa

doctor: build
	/tmp/fmqa doctor --factorio-bin $(FACTORIO_BIN)

validate-fixture: build
	/tmp/fmqa validate --snapshot fixtures/prototype_snapshots/qa_broken_mod.json --reports-dir /tmp/fmqa-reports
