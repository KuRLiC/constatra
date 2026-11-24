BINARY := constatra
BUILD_DIR := build
VERSION ?= dev
GO ?= go
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build build-all test check clean

build:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/constatra

build-all:
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/constatra
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/constatra

test:
	$(GO) test ./...

check:
	$(GO) vet ./...
	$(GO) test -race ./...

clean:
	rm -rf $(BUILD_DIR)
