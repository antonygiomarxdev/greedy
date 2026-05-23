APP_NAME := greedy
BUILD_DIR := build
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/antonygiomarxdev/greedy/internal/version.Version=$(VERSION) -X github.com/antonygiomarxdev/greedy/internal/version.Commit=$(COMMIT) -X github.com/antonygiomarxdev/greedy/internal/version.Date=$(DATE)

.PHONY: build build-all build-linux build-darwin build-windows clean test test-verbose run install-service release install

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

install:
ifeq ($(shell uname -s),Linux)
	$(eval OS := linux)
else ifeq ($(shell uname -s),Darwin)
	$(eval OS := darwin)
else
	$(error Unsupported OS: $(shell uname -s))
endif
	$(eval ARCH := $(shell uname -m | sed 's/aarch64/arm64/;s/x86_64/amd64/'))
	$(eval VERSION_TAG := $(shell curl -s https://api.github.com/repos/antonygiomarxdev/greedy/releases/latest | grep '"tag_name":' | sed -E 's/.*"v([^"]+)".*/\1/'))
	@echo "Downloading greedy version $(VERSION_TAG) for $(OS)/$(ARCH)..."
	curl -fsSL "https://github.com/antonygiomarxdev/greedy/releases/latest/download/greedy_$(VERSION_TAG)_$(OS)_$(ARCH).tar.gz" \
	  -o "greedy_$(VERSION_TAG)_$(OS)_$(ARCH).tar.gz"
	tar -xzf "greedy_$(VERSION_TAG)_$(OS)_$(ARCH).tar.gz" greedy
	install greedy /usr/local/bin/greedy
	rm -f "greedy_$(VERSION_TAG)_$(OS)_$(ARCH).tar.gz" greedy
	@echo "Installed: $$(greedy version 2>/dev/null || true)"
