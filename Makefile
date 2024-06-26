.DEFAULT_GOAL := build
LOCAL_BIN ?= $(shell pwd)/bin
BINARY ?= $(LOCAL_BIN)/makefile-graph

$(LOCAL_BIN):
	mkdir -p $(LOCAL_BIN)

$(BINARY): $(LOCAL_BIN)
	go build -o $(BINARY) cmd/makefile-graph/main.go

build: $(BINARY)

get:
	go get -v -t -d ./...

test:
	go test -v -race $(shell go list ./... | grep -v -E 'cmd|fixtures')

test-cover:
	go test -v -race -coverprofile=coverage.txt -covermode=atomic $(shell go list ./... | grep -v -E 'cmd|fixtures')

.PHONY: get test test-cover build
