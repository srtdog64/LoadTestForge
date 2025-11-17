.PHONY: build run test clean docker-build docker-run help

BINARY_NAME=loadtest
DOCKER_IMAGE=loadtest:latest

help:
	@echo "LoadTestForge Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  run          - Run with default settings"
	@echo "  test         - Run tests"
	@echo "  clean        - Clean build artifacts"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run in Docker"
	@echo "  deps         - Download dependencies"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter"

deps:
	go mod download
	go mod tidy

build: deps
	go build -o $(BINARY_NAME) ./cmd/loadtest

build-linux: deps
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux ./cmd/loadtest

run: build
	./$(BINARY_NAME) \
		--target http://httpbin.org/get \
		--strategy normal \
		--sessions 100 \
		--rate 10 \
		--duration 30s

test:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

bench:
	go test -bench=. -benchmem ./...

fmt:
	go fmt ./...
	gofmt -s -w .

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-linux
	rm -f coverage.out coverage.html
	rm -rf dist/ build/

docker-build:
	docker build -t $(DOCKER_IMAGE) -f deployments/docker/Dockerfile .

docker-run: docker-build
	docker run --rm $(DOCKER_IMAGE) \
		--target http://httpbin.org/get \
		--strategy normal \
		--sessions 100 \
		--rate 10 \
		--duration 30s

docker-compose-up:
	docker-compose -f deployments/docker/docker-compose.yml up

docker-compose-down:
	docker-compose -f deployments/docker/docker-compose.yml down

aws-deploy:
	bash deployments/aws/deploy.sh

install: build
	sudo cp $(BINARY_NAME) /usr/local/bin/

uninstall:
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
