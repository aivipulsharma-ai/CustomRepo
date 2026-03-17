.PHONY: build clean test run

# Build the application
build:
	go build -o dextr-app .

# Clean build artifacts
clean:
	rm -f dextr-app

# Run tests
test:
	go test ./...

# Run the application
run: build
	./dextr-app startapp

# Install dependencies
deps:
	go mod tidy
	go mod download

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Build for different platforms
build-linux:
	GOOS=linux GOARCH=amd64 go build -o dextr-app-linux .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o dextr-app-darwin .

build-windows:
	GOOS=windows GOARCH=amd64 go build -o dextr-app-windows.exe .

# Build all platforms
build-all: build-linux build-darwin build-windows 