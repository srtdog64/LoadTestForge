# LoadTestForge TODO

## Integration Test Scenarios

### Red Team Scenario: "The Shark Fin" (상어 지느러미 패턴)

**Purpose:** Validate rapid burst (Scale UP) and pruning (Scale DOWN) without goroutine leaks.

**Command:**
```bash
./loadtest \
  --target http://127.0.0.1:8080 \
  --sessions 2000 \
  --rate 500 \
  --pulse \
  --pulse-wave square \
  --pulse-high 10s \
  --pulse-low 10s \
  --pulse-ratio 0.1 \
  --duration 1m
```

**Validation Checklist:**

- [ ] **Scale UP (10s High Phase)**
  - [ ] Sessions reach 2,000 within ~4 seconds (rate=500/s)
  - [ ] Non-blocking: control loop maintains 50ms tick precision
  - [ ] No "stuck" goroutines during rapid spawn

- [ ] **Scale DOWN (10s Low Phase)**
  - [ ] Sessions drop to 200 (10% of 2000)
  - [ ] Active Pruning triggers correctly
  - [ ] Damping factor prevents overshooting below 200

- [ ] **Stability Over Multiple Cycles**
  - [ ] 6 full cycles in 1 minute (High->Low->High->Low->High->Low)
  - [ ] Active Goroutines follows target accurately (±10%)
  - [ ] No memory leaks (stable RSS over time)
  - [ ] No goroutine leaks (`runtime.NumGoroutine()` stable)

- [ ] **Metrics Accuracy**
  - [ ] TCP Connections matches Active Sessions (±10%)
  - [ ] Success/Failure counts are consistent
  - [ ] No panic or runtime errors

---

### Scenario: "The Tsunami" (쓰나미 패턴)

**Purpose:** Validate extreme scale-up from minimal load.

**Command:**
```bash
./loadtest \
  --target http://127.0.0.1:8080 \
  --sessions 5000 \
  --rate 1000 \
  --pulse \
  --pulse-wave square \
  --pulse-high 30s \
  --pulse-low 30s \
  --pulse-ratio 0.02 \
  --duration 3m
```

**Validation Checklist:**

- [ ] **Low Phase (100 sessions = 2% of 5000)**
  - [ ] Stable at 100 sessions

- [ ] **High Phase (5000 sessions)**
  - [ ] Reaches target within 5 seconds
  - [ ] Control loop not blocked (phase transitions on time)

- [ ] **Memory Profile**
  - [ ] Peak memory during High phase
  - [ ] Memory released during Low phase
  - [ ] No OOM under 1GB limit

---

### Scenario: "The Sawtooth Stress" (톱니 스트레스)

**Purpose:** Validate gradual ramp-up with instant drop.

**Command:**
```bash
./loadtest \
  --target http://127.0.0.1:8080 \
  --sessions 3000 \
  --rate 300 \
  --pulse \
  --pulse-wave sawtooth \
  --pulse-high 1m \
  --pulse-low 5s \
  --pulse-ratio 0.1 \
  --duration 5m
```

**Validation Checklist:**

- [ ] **Gradual Rise (1m High Phase)**
  - [ ] Sessions increase linearly from 300 to 3000
  - [ ] Smooth progression visible in metrics

- [ ] **Instant Drop (5s Low Phase)**
  - [ ] Sessions drop to 300 within 2-3 seconds
  - [ ] Active Pruning handles 2700 session terminations
  - [ ] Server receives connection close signals (Broken Pipe logs)

---

### Scenario: "The Sine Wave" (사인파 패턴)

**Purpose:** Validate smooth oscillation for auto-scaling hysteresis testing.

**Command:**
```bash
./loadtest \
  --target http://127.0.0.1:8080 \
  --sessions 1000 \
  --rate 200 \
  --pulse \
  --pulse-wave sine \
  --pulse-high 30s \
  --pulse-low 30s \
  --pulse-ratio 0.2 \
  --duration 3m
```

**Validation Checklist:**

- [ ] **Smooth Oscillation**
  - [ ] Sessions follow sinusoidal curve (no step-wise jumps)
  - [ ] Peak at 1000, trough at 200

- [ ] **Auto-scaling Correlation**
  - [ ] If target has auto-scaling, observe scale-up/down lag
  - [ ] Document hysteresis behavior

---

## Strategy-Specific Tests

### H2 Flood Validation

- [ ] HTTP/2 negotiation succeeds (ALPN)
- [ ] Multiple streams on single connection
- [ ] `--max-streams 100` creates 100 concurrent streams
- [ ] Falls back to h2c for non-TLS targets

### Heavy Payload Validation

- [ ] `deep-json`: 50-level nested JSON parses on server
- [ ] `redos`: Server CPU spikes on regex patterns
- [ ] `nested-xml`: XML parser handles deep nesting
- [ ] `query-flood`: URL parser handles 1000+ params
- [ ] `multipart`: Form handler processes large multipart

---

## Performance Benchmarks

### Baseline Metrics (to be filled)

| Scenario | Target Sessions | Actual Sessions | Memory Peak | CPU Peak |
|----------|-----------------|-----------------|-------------|----------|
| Shark Fin | 2000 | | | |
| Tsunami | 5000 | | | |
| Sawtooth | 3000 | | | |
| Sine Wave | 1000 | | | |

---

## Future Improvements

- [ ] TLS Fingerprint Randomization (JA3 Spoofing)
- [ ] Scenario-based testing (Login -> Token -> API flow)
- [ ] Chaos Metrics feedback loop (auto-adjust based on 5xx rate)
- [ ] Event-driven I/O (`gnet` library for 100k+ connections)
- [ ] Distributed mode (multiple nodes coordinated attack)
