.PHONY: build test clean

# The name of the output binary
BINARY_NAME=augur

# The Go path
GOPATH=$(shell go env GOPATH)

# The build commands
GOBUILD=go build
GOTEST=go test
GOCLEAN=go clean
GOGET=go get
GOMODTIDY=go mod tidy
GOMODVENDOR=go mod vendor
GOINSTALL=go install

build:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/augur

install:
	$(GOBUILD) -o $(BINARY_NAME) -v ./cmd/augur
	mv augur $(GOPATH)/bin

all: clean deps test build

.PHONY: app-up
app-up:
	docker-compose -p augur --profile augur up

.PHONY: app-down
app-down:
	docker-compose -p augur --profile augur down
