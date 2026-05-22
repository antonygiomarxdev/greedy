APP_NAME := greedy
BUILD_DIR := build
LDFLAGS := -s -w

.PHONY: build build-all clean test run

build:
	go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/greedy

build-all: build-linux build-darwin build-windows

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 ./cmd/greedy

build-darwin:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/greedy

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/greedy

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./... -count=1 -timeout=30s

test-verbose:
	go test ./... -count=1 -timeout=30s -v

run: build
	./$(BUILD_DIR)/$(APP_NAME) $(ARGS)
