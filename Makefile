.DEFAULT_GOAL := run

# Get the default Google Cloud Project ID
GOOGLE_PROJECT_ID ?= $(shell gcloud config get-value project 2>/dev/null)

# Set the function target
FUNCTION_TARGET ?= RESTHandler

# Flags
LOCAL_ONLY ?= true

# Run the API locally
run:
	@echo "Running API with project: $(GOOGLE_PROJECT_ID)"
	@FUNCTION_TARGET=$(FUNCTION_TARGET) LOCAL_ONLY=$(LOCAL_ONLY) GOOGLE_PROJECT_ID=$(GOOGLE_PROJECT_ID) go run cmd/main.go

# Clean Go build cache
clean:
	@go clean -cache -modcache -testcache -i
	@echo "Cleaned Go build cache."

# Help command
help:
	@echo "Makefile Commands:"
	@echo "  make run       - Runs the API with environment variables"
	@echo "  make clean     - Cleans Go build cache"
	@echo "  make help      - Shows this help message"


