# LoadTestForge Usage Guide

## Quick Start

### Basic Load Test

```bash
./loadtest --target http://example.com --sessions 100 --rate 10
```

### Slowloris Attack Simulation

```bash
./loadtest \
  --target http://example.com \
  --strategy slowloris \
  --sessions 5000 \
  --rate 500 \
  --keepalive 15s \
  --duration 10m
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | (required) | Target URL |
| `--strategy` | `normal` | Attack strategy (`normal` or `slowloris`) |
| `--sessions` | `100` | Target concurrent sessions |
| `--rate` | `10` | Sessions per second to create |
| `--duration` | `0` (infinite) | Test duration (e.g., `30s`, `5m`, `1h`) |
| `--method` | `GET` | HTTP method |
| `--timeout` | `10s` | Request timeout |
| `--keepalive` | `10s` | Slowloris keep-alive interval |

### Advanced Options

| Flag | Default | Description |
|------|---------|-------------|
| `--stealth` | `false` | Enable browser fingerprint headers (Sec-Fetch-*) for WAF bypass |
| `--randomize` | `false` | Enable realistic query strings for cache bypass |
| `--analyze-latency` | `false` | Enable response time percentile analysis (p50, p95, p99) |

## Examples

### 1. Simple HTTP Load Test

Test a website with 1,000 concurrent users making 100 requests per second:

```bash
./loadtest \
  --target http://your-site.com \
  --sessions 1000 \
  --rate 100 \
  --duration 5m
```

### 2. POST Request Load Test

```bash
./loadtest \
  --target http://api.example.com/data \
  --method POST \
  --sessions 500 \
  --rate 50
```

### 3. Slowloris Simulation

Simulate a Slowloris attack with 5,000 slow connections:

```bash
./loadtest \
  --target http://target.com \
  --strategy slowloris \
  --sessions 5000 \
  --rate 1000 \
  --keepalive 20s
```

### 4. High-Performance Test

Maximum throughput test (adjust based on your system):

```bash
./loadtest \
  --target http://target.com \
  --sessions 10000 \
  --rate 2000 \
  --duration 10m
```

## Understanding the Output

### Live Stats Display

```
=== LoadTestForge Live Stats ===
Elapsed Time: 1m30s

Active Sessions: 1000
Total Requests:  150000
Success:         149850 (99.90%)
Failed:          150

Requests/sec:    998.50 (σ=45.23)
Min/Max:         920 / 1080

Deviation:       4.53%
Status:          ✓ Within target (±10%)
```

### Metrics Explained

- **Active Sessions**: Current number of concurrent connections
- **Total Requests**: Cumulative number of requests sent
- **Success/Failed**: Request outcomes with success percentage
- **Requests/sec**: Average requests per second with standard deviation (σ)
- **Min/Max**: Minimum and maximum requests in any given second
- **Deviation**: Percentage deviation from the average
- **Status**: Whether the rate control is within ±10% target

## Performance Tuning

### System Limits

Before running high-load tests, increase system limits:

**Linux:**
```bash
ulimit -n 65536
sysctl -w net.ipv4.ip_local_port_range="1024 65535"
sysctl -w net.ipv4.tcp_tw_reuse=1
```

**Docker:**
```yaml
deploy:
  resources:
    limits:
      cpus: '2'
      memory: 2G
ulimits:
  nofile:
    soft: 65536
    hard: 65536
```

### Expected Performance

On a modern system (4 CPU cores, 8GB RAM):

| Sessions | Rate | Memory | CPU |
|----------|------|--------|-----|
| 100 | 10/s | ~20MB | <5% |
| 1,000 | 100/s | ~100MB | 15-25% |
| 5,000 | 500/s | ~300MB | 40-60% |
| 10,000 | 1000/s | ~600MB | 80-100% |

## AWS Deployment

### One-Time Test

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
        "--target", "http://your-target.com",
        "--sessions", "5000",
        "--rate", "500",
        "--duration", "10m"
      ]
    }]
  }'
```

### View Logs

```bash
aws logs tail /ecs/loadtest --follow
```

### Clean Up

```bash
aws ecs list-tasks --cluster loadtest-cluster
aws ecs stop-task --cluster loadtest-cluster --task <task-id>
```

## Best Practices

### 1. Start Small

Always start with low values and gradually increase:

```bash
# Start
./loadtest --target http://example.com --sessions 10 --rate 1

# Gradually increase
./loadtest --target http://example.com --sessions 100 --rate 10
./loadtest --target http://example.com --sessions 1000 --rate 100
```

### 2. Monitor Target System

Watch your target system's:
- CPU usage
- Memory usage
- Network bandwidth
- Response times
- Error rates

### 3. Rate Control Accuracy

The tool aims for ±10% accuracy in session creation rate. Factors affecting accuracy:
- OS scheduler precision
- Network latency
- Target server performance
- Available system resources

### 4. Slowloris Considerations

Slowloris attacks:
- Use TCP connections efficiently
- Require proper timeout settings on target
- May be blocked by WAF/DDoS protection
- Should only be used on systems you own or have permission to test

### 5. Legal Compliance

Always ensure you have:
- Written permission to test the target system
- Proper authorization from system owners
- Understanding of local laws regarding load testing
- Notification to affected parties if required

## Troubleshooting

### Issue: "Too many open files"

**Solution:**
```bash
ulimit -n 65536
```

### Issue: Rate fluctuates wildly (>20% deviation)

**Possible causes:**
1. Target server is overloaded
2. Network congestion
3. Insufficient system resources

**Solutions:**
- Reduce `--sessions` or `--rate`
- Check target server capacity
- Monitor network bandwidth
- Increase timeout: `--timeout 30s`

### Issue: High failure rate

**Check:**
1. Target server is reachable
2. Firewall rules allow traffic
3. Rate limiting on target side
4. DNS resolution issues

### Issue: Memory usage too high

**Solutions:**
- Reduce concurrent sessions
- Increase `--timeout` to release connections faster
- Check for memory leaks (should be stable over time)

## Advanced Usage

### Stealth Mode (WAF Bypass)

Enable browser fingerprint headers to bypass Web Application Firewalls:

```bash
./loadtest \
  --target http://example.com \
  --strategy http-flood \
  --sessions 500 \
  --rate 50 \
  --stealth
```

Stealth mode adds:
- `Sec-Fetch-Dest`, `Sec-Fetch-Mode`, `Sec-Fetch-Site`, `Sec-Fetch-User` headers
- `Sec-CH-UA` client hints (Chrome/Edge browser fingerprint)
- Randomized `X-Forwarded-For` headers (50% probability)
- Randomized `X-Real-IP` headers (30% probability)

### Cache Bypass (Randomize Mode)

Enable realistic query strings to bypass CDN/cache servers:

```bash
./loadtest \
  --target http://example.com \
  --strategy http-flood \
  --sessions 500 \
  --rate 50 \
  --randomize
```

Randomize mode adds realistic query parameters:
- Timestamp (`_=1234567890123`)
- Random number (`r=0.12345678`)
- Referrer source (`ref=google|naver|facebook|...`)
- Version (`v=1-100`)
- User ID (20% probability, `uid=10000-99999`)
- Session ID (15% probability)
- Device type (25% probability)
- UTM source (10% probability)

### Latency Analysis

Enable response time percentile analysis for QoS testing:

```bash
./loadtest \
  --target http://example.com \
  --strategy http-flood \
  --sessions 200 \
  --rate 20 \
  --analyze-latency
```

Latency analysis provides:
- Sample count
- Average response time (ms)
- Min/Max response time (ms)
- p50, p95, p99 percentiles (ms)

### Combined Advanced Options

For comprehensive testing with all advanced features:

```bash
./loadtest \
  --target http://example.com \
  --strategy http-flood \
  --sessions 500 \
  --rate 50 \
  --duration 5m \
  --stealth \
  --randomize \
  --analyze-latency
```

### Custom Headers

Currently not supported via CLI. For custom headers, modify the code in `cmd/loadtest/main.go`:

```go
target := strategy.Target{
    URL:    cfg.Target.URL,
    Method: cfg.Target.Method,
    Headers: map[string]string{
        "Authorization": "Bearer token",
        "Content-Type":  "application/json",
    },
}
```

### POST Body

For POST requests with body (future enhancement):

```go
target := strategy.Target{
    URL:    cfg.Target.URL,
    Method: "POST",
    Body:   []byte(`{"key": "value"}`),
}
```

## Contributing

To add new attack strategies:

1. Implement the `AttackStrategy` interface in `internal/strategy/`
2. Add strategy creation logic in `cmd/loadtest/main.go`
3. Update documentation
4. Add tests

Example:

```go
type CustomStrategy struct{}

func (c *CustomStrategy) Execute(ctx context.Context, target Target) error {
    // Implementation
    return nil
}

func (c *CustomStrategy) Name() string {
    return "custom"
}
```
