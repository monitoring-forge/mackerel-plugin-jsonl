VERSION=0.0.10
LDFLAGS=-ldflags "-w -s -X main.version=${VERSION}"
all: mackerel-plugin-jsonl

.PHONY: mackerel-plugin-jsonl linux check lint bench

mackerel-plugin-jsonl: cmd/mackerel-plugin-jsonl/*.go
	go build $(LDFLAGS) -o mackerel-plugin-jsonl ./cmd/mackerel-plugin-jsonl

linux: cmd/mackerel-plugin-jsonl/*.go
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o mackerel-plugin-jsonl ./cmd/mackerel-plugin-jsonl

check:
	go test -v ./...
	go test -race ./...

lint:
	golangci-lint run --timeout 5m ./...

bench:
	go test -bench BenchmarkMainParse -benchmem -run '^$$' ./...