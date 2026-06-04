# AI Novel Agent — Makefile
# Cross-compiles the Go binary for all target platforms.

APP_NAME    = novel-agent
VERSION    ?= 1.0.0-alpha
LDFLAGS    := -s -w -X main.version=$(VERSION)
DIST_DIR   := dist
CMD_DIR    := ./cmd/novel-agent/

# Build targets for each platform
PLATFORMS := \
	darwin/amd64 \
	darwin/arm64 \
	linux/amd64 \
	windows/amd64

.PHONY: all build clean test vet lint

all: build

build:
	@echo "Building $(APP_NAME) v$(VERSION)..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} \
		GOARCH=$${platform#*/} \
		OUT="$(DIST_DIR)/$(APP_NAME)_$${GOOS}_$${GOARCH}"; \
		if [ "$$GOOS" = "windows" ]; then OUT="$$OUT.exe"; fi; \
		echo "  → $$OUT"; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags "$(LDFLAGS)" -o "$$OUT" $(CMD_DIR) || exit 1; \
	done
	@echo "✓ Build complete — binaries in $(DIST_DIR)/"

test:
	go test ./internal/... -v -cover

vet:
	go vet ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(DIST_DIR)

# --- npm packaging helpers ---

npm-pack: build
	@echo "Packaging for npm..."
	@mkdir -p npm/dist
	@cp $(DIST_DIR)/* npm/dist/
	@echo "✓ npm/dist/ populated"
