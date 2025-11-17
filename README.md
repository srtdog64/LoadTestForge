# LoadTestForge

High-performance load testing tool with Slowloris attack support.

## Features

- Normal HTTP load testing
- Slowloris attack simulation
- Precise rate control (sessions per second)
- Real-time metrics with statistical analysis
- Single binary deployment
- AWS-ready architecture

## Architecture

```
LoadTestForge/
├── cmd/
│   └── loadtest/           # CLI entry point
├── internal/
│   ├── strategy/           # Attack strategies
│   ├── session/            # Session management
│   ├── metrics/            # Metrics collection
│   └── config/             # Configuration
├── deployments/
│   ├── docker/             # Docker setup
│   └── aws/                # AWS deployment configs
└── docs/                   # Documentation
```

## Development Roadmap

### Phase 1: Foundation (Week 1)
- [x] Project structure
- [ ] Go module initialization
- [ ] Basic HTTP strategy
- [ ] Rate limiter

### Phase 2: Core Features (Week 2)
- [ ] Session manager
- [ ] Slowloris implementation
- [ ] Metrics collector

### Phase 3: Polish (Week 3)
- [ ] CLI interface
- [ ] Config file support
- [ ] Real-time reporting

### Phase 4: Deployment (Week 4)
- [ ] Docker container
- [ ] AWS EC2/ECS setup
- [ ] Documentation

## Quick Start

```bash
# Build
go build -o loadtest cmd/loadtest/main.go

# Run
./loadtest --target http://example.com --strategy normal --sessions 1000 --rate 100
```

## AWS Deployment

```bash
# Docker build
docker build -t loadtest:latest -f deployments/docker/Dockerfile .

# Push to ECR
aws ecr get-login-password | docker login --username AWS --password-stdin {ecr-url}
docker tag loadtest:latest {ecr-url}/loadtest:latest
docker push {ecr-url}/loadtest:latest

# Deploy to ECS
aws ecs update-service --cluster loadtest-cluster --service loadtest-service --force-new-deployment
```

## Performance Targets

- 5,000+ concurrent sessions
- <10% standard deviation on session rate
- <200MB memory footprint
- Single CPU core capable of 1,000 sessions/sec

## Legal Notice

This tool is for authorized load testing only. Unauthorized use against systems you do not own or have permission to test is illegal.
