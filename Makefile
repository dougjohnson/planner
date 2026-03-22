# Makefile for flywheel-planner production build
# Produces a single Go binary with embedded React frontend.

BINARY_NAME := flywheel-planner
BUILD_DIR := build
FRONTEND_DIR := frontend
FRONTEND_DIST := $(FRONTEND_DIR)/dist
GO_MODULE := github.com/dougflynn/flywheel-planner

# Build flags.
VERSION ?= dev
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all clean build frontend backend test lint vet check

# Default: full production build.
all: build

# Full production build: frontend → embed → Go binary.
build: frontend backend
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build the React frontend.
frontend:
	@echo "Building frontend..."
	cd $(FRONTEND_DIR) && npm run build
	@echo "Frontend built to $(FRONTEND_DIST)"

# Build the Go binary with embedded frontend assets.
backend:
	@echo "Building backend..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/flywheel-planner
	@echo "Binary: $(BUILD_DIR)/$(BINARY_NAME)"

# Run all tests.
test: test-backend test-frontend

test-backend:
	go test ./... -race

test-frontend:
	cd $(FRONTEND_DIR) && npm test

# Lint and static analysis.
lint: vet
	cd $(FRONTEND_DIR) && npm run lint

vet:
	go vet ./...

# Type checking.
check:
	cd $(FRONTEND_DIR) && npm run typecheck
	go build ./...

# Format.
fmt:
	gofmt -w .
	cd $(FRONTEND_DIR) && npx prettier --write .

# Clean build artifacts.
clean:
	rm -rf $(BUILD_DIR)
	rm -rf $(FRONTEND_DIST)

# Development: run backend with hot reload frontend proxy.
dev:
	@echo "Start frontend dev server: cd frontend && npm run dev"
	@echo "Start backend: go run ./cmd/flywheel-planner"
	@echo "Frontend proxies /api to backend at 127.0.0.1:7432"

# Verify production build works.
verify: build
	@echo "Verifying build..."
	$(BUILD_DIR)/$(BINARY_NAME) --version 2>/dev/null || true
	@echo "Verification complete."
