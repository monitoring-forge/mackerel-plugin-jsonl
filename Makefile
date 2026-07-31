VERSION=0.0.8
GITCOMMIT?=$(shell git describe --dirty --always)
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION} -X main.commit=${GITCOMMIT}"
all: mackerel-plugin-jsonl

.PHONY: mackerel-plugin-jsonl

mackerel-plugin-jsonl: cmd/mackerel-plugin-jsonl/*.go
	go build $(LDFLAGS) -o mackerel-plugin-jsonl ./cmd/mackerel-plugin-jsonl

linux: cmd/mackerel-plugin-jsonl/*.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o mackerel-plugin-jsonl ./cmd/mackerel-plugin-jsonl

check:
	go test -v ./...
	go test -race ./...

lint:
	golangci-lint run --timeout 5m ./...

