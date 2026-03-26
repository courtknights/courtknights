BINARY_NAME := courtknights-api
BUILD_DIR := build
CMD_SERVER := ./cmd/server

.PHONY: all build run test test-int test-all test-cov test-int-cov lint fmt clean

all: build

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_SERVER)

run:
	go run $(CMD_SERVER)

test:
	go test -race -count=1 ./...

test-int:
	go test -race -count=1 -tags integration ./...

test-all: test test-int

# COV_OUT must be set to an absolute path for the coverage profile output.
test-cov:
	go test -race -count=1 -coverprofile=$(COV_OUT) -covermode=atomic ./...

test-int-cov:
	go test -race -count=1 -tags=integration -coverprofile=$(COV_OUT) -covermode=atomic ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

clean:
	rm -rf $(BUILD_DIR)/$(BINARY_NAME)
