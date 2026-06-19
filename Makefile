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
