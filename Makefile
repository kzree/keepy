# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run/keepy: run the cmd/keepy application
.PHONY: run/keepy
run/keepy:
	@go run ./cmd/keepy

## test: run all tests
.PHONY: test
test:
	@go test ./...

## test/ci: run all tests and generate a coverage profile
.PHONY: test/ci
test/ci:
	@go test -coverprofile=coverage.txt ./...

## install-hooks: configure git to use repository hooks
.PHONY: install-hooks
install-hooks:
	@git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit .githooks/commit-msg

# ==================================================================================== #
# BUILD
# ==================================================================================== #
APP_NAME := keepy
BUILD_DIR := bin

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: build
build:
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		EXT=""; \
		if [ "$$GOOS" = "windows" ]; then EXT=".exe"; fi; \
		OUT="$(BUILD_DIR)/$(APP_NAME)-$$GOOS-$$GOARCH$$EXT"; \
		echo "Building $$OUT..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -o $$OUT ./cmd/keepy || exit 1; \
	done
