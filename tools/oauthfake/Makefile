.PHONY: build test lint

build:
	go build -o build/oauthfake .

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...
