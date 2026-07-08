PORT    ?= 8080
SCHEMA  ?= testdata/schemas/ecommerce.sql
ROWS    ?= 10
OUTPUT  ?= /dev/stdout
FORMAT  ?= sql
SEED    ?= 42

.PHONY: build-cli build-web build-all build run-web run-cli test test-quick test-server lint clean ci

build-cli:
	CGO_ENABLED=1 go build -o bin/synthgraph ./cmd/synthgraph/

build-web:
	CGO_ENABLED=1 go build -o bin/synthgraph-web ./cmd/synthgraph-web/

build-all: build-cli build-web

build: build-all

run-web:
	CGO_ENABLED=1 go run ./cmd/synthgraph-web/ --port $(PORT)

run-cli:
	CGO_ENABLED=1 go run ./cmd/synthgraph/ generate --input $(SCHEMA) --rows $(ROWS) --output $(OUTPUT) --format $(FORMAT) --seed $(SEED)

test:
	CGO_ENABLED=1 go test ./... -count=1

test-quick:
	go test ./internal/... ./cmd/synthgraph/... ./cmd/synthgraph-web/server/... -count=1

test-server:
	go test ./cmd/synthgraph-web/server/... -v -count=1

lint:
	go vet ./...

clean:
	rm -rf bin/ coverage.out

ci: lint build-all test
