APP_NAME := greedy
BUILD_DIR := build
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/antonygiomarxdev/greedy/internal/version.Version=$(VERSION) -X github.com/antonygiomarxdev/greedy/internal/version.Commit=$(COMMIT) -X github.com/antonygiomarxdev/greedy/internal/version.Date=$(DATE)

.PHONY: build build-all build-linux build-darwin build-windows clean test test-verbose run install-service release

build:
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/greedy

build-all: build-linux build-darwin build-windows

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/greedy

build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/greedy

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/greedy

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./... -count=1 -timeout=30s

test-verbose:
	go test ./... -count=1 -timeout=30s -v

run: build
	./$(BUILD_DIR)/$(APP_NAME) $(ARGS)

install-service: build
	cp $(BUILD_DIR)/$(APP_NAME) /usr/local/bin/$(APP_NAME)
	cp greedy.service /etc/systemd/system/$(APP_NAME).service
	systemctl daemon-reload
	@echo "Service installed. Run: sudo systemctl enable $(APP_NAME) && sudo systemctl start $(APP_NAME)"

release: clean build-all
	@echo "Release binaries in $(BUILD_DIR)/"
	ls -lh $(BUILD_DIR)/
