.PHONY: dev build clean deps frontend-deps

# Development — hot-reload both Go and Vue
dev:
	cd backend && wails dev

# Production build
build:
	cd backend && wails build

# Install all dependencies
deps: frontend-deps
	go mod tidy

frontend-deps:
	cd view && npm install

# Clean build artifacts
clean:
	rm -rf build/bin
	rm -rf view/dist
	rm -rf view/node_modules

# Run Go tests
test:
	go test ./...

# Lint
lint:
	golangci-lint run ./...
