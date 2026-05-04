.PHONY: build test vet lint sec all clean

BINARY := bin/vault-tui

build:
	go build -o $(BINARY) ./cmd/vault-tui

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

sec:
	gosec ./...

all: vet test build

clean:
	rm -rf bin
