# LoadTestForge

High-performance load testing tool with Slowloris attack support and advanced metrics.

## Features

- **Multiple Attack Strategies**
  - Normal HTTP load testing
  - Slowloris attack simulation with User-Agent randomization
  - HTTPS/TLS support with proper certificate validation

- **Precise Rate Control**
  - Token bucket algorithm for accurate rate limiting
  - Ramp-up support for gradual load increase
  - Target: ±10% standard deviation

- **Advanced Metrics**
  - Real-time statistics
  - Percentile analysis (p50, p95, p99)
  - Standard deviation tracking
  - Success rate monitoring

- **Production Ready**
  - Session lifetime limits (5min max)
  - Graceful shutdown
  - Context-based cancellation
  - Connection pooling

- **AWS Deployment**
  - Docker containerization
  - ECS Fargate support
  - CloudWatch integration

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/jdw/LoadTestForge.git
cd LoadTestForge

# Build
go build -o loadtest ./cmd/loadtest

# Or use make
make build
```

### Basic Usage

```bash
# Simple HTTP load test
./loadtest --target http://httpbin.org/get --sessions 100 --rate 10 --duration 1m

# With ramp-up
./loadtest --target http://example.com --sessions 1000 --rate 100 --rampup 30s --duration 5m

# Slowloris simulation
./loadtest --target http://example.com --strategy slowloris --sessions 500 --rate 50
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | (required) | Target URL (http:// or https://) |
| `--strategy` | `normal` | Attack strategy (`normal` or `slowloris`) |
| `--sessions` | `100` | Target concurrent sessions |
| `--rate` | `10` | Sessions per second to create |
| `--duration` | `0` (infinite) | Test duration (e.g., `30s`, `5m`, `1h`) |
| `--rampup` | `0` | Ramp-up duration for gradual load increase |
| `--method` | `GET` | HTTP method |
| `--timeout` | `10s` | Request timeout |
| `--keepalive` | `10s` | Slowloris keep-alive interval |

## Examples

### 1. Gradual Load Test with Ramp-up

```bash
# Start at 0, reach 1000 sessions over 2 minutes, then maintain
./loadtest \
  --target https://api.example.com \
  --sessions 1000 \
  --rate 100 \
  --rampup 2m \
  --duration 10m
```

### 2. Stress Test with Percentile Analysis

```bash
# High load with detailed metrics
./loadtest \
  --target http://your-site.com \
  --sessions 5000 \
  --rate 500 \
  --duration 5m
```

Output:
```
=== LoadTestForge Live Stats ===
Elapsed Time: 2m30s

Active Sessions: 5000
Total Requests:  750000
Success:         749500 (99.93%)
Failed:          500

Requests/sec:    4998.50 (σ=45.23)
Min/Max:         4920 / 5080
Percentiles:     p50=5000, p95=5050, p99=5070

Deviation:       0.90%
Status:          ✓ Within target (±10%)
```

### 3. HTTPS Slowloris Test

```bash
# HTTPS with TLS handshake and random User-Agents
./loadtest \
  --target https://secure-site.com \
  --strategy slowloris \
  --sessions 1000 \
  --rate 200 \
  --keepalive 15s
```

### 4. Spike Test

```bash
# Instant spike to 10000 sessions
./loadtest \
  --target http://cdn.example.com \
  --sessions 10000 \
  --rate 2000 \
  --duration 30s
```

## Performance Targets

On a modern system (4 CPU cores, 8GB RAM):

| Sessions | Rate | Memory | CPU | Ramp-up Time |
|----------|------|--------|-----|--------------|
| 100 | 10/s | ~20MB | <5% | N/A |
| 1,000 | 100/s | ~100MB | 15-25% | 10s |
| 5,000 | 500/s | ~300MB | 40-60% | 30s |
| 10,000 | 1000/s | ~600MB | 80-100% | 1m |

## Understanding Metrics

### Percentiles (p50, p95, p99)

- **p50 (Median)**: 50% of requests were at or below this rate
- **p95**: 95% of requests were at or below this rate (typical SLA)
- **p99**: 99% of requests were at or below this rate (tail latency)

### Why Percentiles Matter

Average can be misleading. Example:

```
Average: 100 req/s
p50: 98 req/s
p95: 150 req/s  ← 5% of seconds had this spike
p99: 200 req/s  ← Rare but important outliers
```

## AWS Deployment

### Quick Deploy

```bash
cd deployments/aws
chmod +x deploy.sh
./deploy.sh
```

### Run One-Time Load Test

```bash
aws ecs run-task \
  --cluster loadtest-cluster \
  --launch-type FARGATE \
  --task-definition loadtest-task \
  --network-configuration "awsvpcConfiguration={
    subnets=[subnet-xxx],
    securityGroups=[sg-xxx],
    assignPublicIp=ENABLED
  }" \
  --overrides '{
    "containerOverrides": [{
      "name": "loadtest",
      "command": [
        "--target", "https://your-target.com",
        "--sessions", "5000",
        "--rate", "500",
        "--rampup", "1m",
        "--duration", "10m"
      ]
    }]
  }'
```

### View Live Logs

```bash
aws logs tail /ecs/loadtest --follow
```

## Docker Usage

### Build and Run Locally

```bash
# Build
docker build -t loadtest:latest -f deployments/docker/Dockerfile .

# Run
docker run --rm loadtest:latest \
  --target http://httpbin.org/get \
  --sessions 100 \
  --rate 10 \
  --duration 1m
```

### Docker Compose

```bash
cd deployments/docker
docker-compose up
```

## Advanced Features

### User-Agent Randomization

Slowloris strategy automatically rotates through 10 realistic User-Agents:
- Chrome (Windows, Mac, Linux)
- Firefox (Windows, Linux)
- Safari (Mac, iOS, iPad)
- Edge (Windows)
- Android Chrome

### Session Lifetime Protection

- Maximum session life: 5 minutes
- Prevents infinite connections
- Automatic cleanup on timeout

### TLS/HTTPS Support

- Proper TLS handshake
- Certificate validation (configurable)
- SNI support
- Works with modern HTTPS servers

## Best Practices

### 1. Always Use Ramp-up for Large Tests

```bash
# Bad: Instant spike might crash target
./loadtest --target http://api.com --sessions 10000 --rate 2000

# Good: Gradual increase
./loadtest --target http://api.com --sessions 10000 --rate 2000 --rampup 2m
```

### 2. Monitor Percentiles, Not Just Average

```
If p99 >> Average, you have:
- Network congestion
- Occasional server overload
- Resource contention
```

### 3. Start Small, Scale Up

```bash
# Step 1: Baseline
./loadtest --target http://api.com --sessions 10 --rate 1 --duration 1m

# Step 2: Increase
./loadtest --target http://api.com --sessions 100 --rate 10 --duration 2m

# Step 3: Scale
./loadtest --target http://api.com --sessions 1000 --rate 100 --rampup 30s --duration 5m
```

### 4. Combine with Target Monitoring

While running LoadTestForge, monitor your target:
- CPU/Memory usage
- Response times
- Error rates
- Database connections
- Network bandwidth

## Troubleshooting

### High p99 values

**Problem:** `p99 >> p50` indicates tail latency issues.

**Solutions:**
- Check target server resources
- Reduce load (`--rate`)
- Increase timeout (`--timeout 30s`)

### "Too many open files"

**Solution:**
```bash
# Linux/Mac
ulimit -n 65536

# Docker
Add to docker-compose.yml:
ulimits:
  nofile:
    soft: 65536
    hard: 65536
```

### Ramp-up not smooth

**Problem:** Load increases in steps instead of smooth curve.

**Cause:** Normal behavior. Manager checks every 100ms.

**Solution:** Use longer ramp-up for smoother curves:
```bash
# Instead of
--rampup 10s

# Use
--rampup 1m
```

## Development

### Run Tests

```bash
make test
```

### Build for Linux

```bash
make build-linux
```

### Format Code

```bash
make fmt
```

## License

MIT License

## Legal Notice

This tool is for authorized load testing only. Unauthorized use against systems you do not own or have permission to test is illegal.

Always:
- Get written permission
- Notify stakeholders
- Understand local laws
- Test in staging first

## Contributing

Pull requests welcome! Please:
1. Add tests for new features
2. Update documentation
3. Follow existing code style
4. Run `make fmt` before committing
