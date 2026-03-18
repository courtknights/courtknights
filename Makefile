BINARY_NAME   := courtknights-api
BUILD_DIR     := build
CMD_SERVER    := ./cmd/server

.PHONY: all build run test test-int test-all lint fmt clean

## all: build the binary (default target)
all: build

## build: compile the backend binary
build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_SERVER)

## run: run the backend server
run:
	go run $(CMD_SERVER)

## test: run unit tests (no external processes required)
test:
	go test -race -count=1 ./...

## test-int: run integration tests (requires Docker for Testcontainers)
test-int:
	go test -race -count=1 -tags integration ./...

## test-all: run unit and integration tests
test-all: test test-int

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## fmt: format all Go files
fmt:
	gofmt -w .

## clean: remove build artefacts
clean:
	rm -rf $(BUILD_DIR)/$(BINARY_NAME)
