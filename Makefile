.PHONY: run test clean build

GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
BINARY_NAME=counter

build:
	$(GOBUILD) -o bin/$(BINARY_NAME) cmd/counter/main.go

run:
	$(GORUN) cmd/counter/main.go

test:
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -f bin/$(BINARY_NAME)

fmt:
	go fmt ./...

lint:
	golangci-lint run

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out