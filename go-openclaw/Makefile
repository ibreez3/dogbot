.PHONY: build run clean test

# Build the gateway binary
build:
	@echo "🔨 Building Gateway..."
	@go build -o bin/gateway cmd/gateway/main.go
	@echo "✅ Build complete: bin/gateway"

# Run the gateway
run:
	@echo "🚀 Starting Gateway..."
	@./bin/gateway

# Clean build artifacts
clean:
	@echo "🧹 Cleaning..."
	@rm -rf bin/
	@echo "✅ Clean complete"

# Run tests
test:
	@echo "🧪 Running tests..."
	@go test ./...

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	@go mod tidy
	@go mod download
	@echo "✅ Dependencies installed"

# Build for multiple platforms
build-all:
	@echo "🔨 Building for multiple platforms..."
	@GOOS=darwin GOARCH=amd64 go build -o bin/gateway-darwin-amd64 cmd/gateway/main.go
	@GOOS=darwin GOARCH=arm64 go build -o bin/gateway-darwin-arm64 cmd/gateway/main.go
	@GOOS=linux GOARCH=amd64 go build -o bin/gateway-linux-amd64 cmd/gateway/main.go
	@GOOS=linux GOARCH=arm64 go build -o bin/gateway-linux-arm64 cmd/gateway/main.go
	@echo "✅ Multi-platform build complete"
