# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run/app: run the cmd/app application
.PHONY: run/app
run/app:
	@go run ./cmd/app

## test: run all tests
.PHONY: test
test:
	@go test ./...

## install-hooks: configure git to use repository hooks
.PHONY: install-hooks
install-hooks:
	@git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit .githooks/commit-msg

# ==================================================================================== #
# BUILD
# ==================================================================================== #
APP_NAME := app
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
		GOOS=$$GOOS GOARCH=$$GOARCH go build -o $$OUT ./cmd/app; \
	done
