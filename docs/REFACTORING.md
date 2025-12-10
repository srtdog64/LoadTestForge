# LoadTestForge 코드 공통화 리팩토링

## 개요

이 문서는 LoadTestForge 프로젝트의 코드 공통화 작업 내역과 향후 개선 방향을 기록합니다.

---

## 완료된 작업

### 1. 상수 통합 (`internal/config/constants.go`)

모든 매직 넘버를 중앙 집중식으로 관리합니다.

| 카테고리 | 상수 예시 | 용도 |
|----------|-----------|------|
| Network | `DefaultConnectTimeout`, `DefaultTCPKeepAlive` | 연결 타임아웃 |
| Session | `SessionTickInterval`, `SpawnBurstMultiplier` | 세션 관리 |
| HTTP | `HTTPSuccessThreshold`, `DefaultUserAgent` | HTTP 처리 |
| Slow Attack | `DefaultReadSize`, `SlowlorisHeaderDelay` | Slow 공격 |
| HTTP/2 | `DefaultMaxStreams`, `H2StreamResetThreshold` | H2 Flood |
| Heavy Payload | `PayloadTypeDeepJSON`, `DefaultPayloadDepth` | Heavy Payload |
| RUDY | `DefaultChunkDelayMin`, `EvasionLevelAggressive` | RUDY 공격 |
| Pulse | `WaveTypeSquare`, `DefaultPulseLowRatio` | Pulse 모드 |
| Metrics | `SuccessRateThreshold`, `P99LatencyThreshold` | 테스트 판정 |
| Backoff | `BaseBackoffDelay`, `MaxBackoffDelay` | 재시도 |
| Buffer | `DefaultReadBufferSize`, `SessionIDLength` | 버퍼 |

**사용 예시:**
```go
// Before
ticker := time.NewTicker(100 * time.Millisecond)

// After
ticker := time.NewTicker(config.SessionTickInterval)
```

---

### 2. BaseStrategy 구조체 (`internal/strategy/base.go`)

모든 전략의 공통 기능을 제공하는 기반 구조체입니다.

**공통 필드:**
```go
type BaseStrategy struct {
    BindConfig        *netutil.BindConfig  // Multi-IP 지원
    activeConnections int64                 // 활성 연결 카운터
    metricsCallback   MetricsCallback       // 메트릭스 콜백
    headerRandomizer  *httpdata.HeaderRandomizer
    enableStealth     bool
    randomizePath     bool
}
```

**제공 메서드:**
- `SetMetricsCallback()` - MetricsAware 인터페이스 구현
- `ActiveConnections()` - ConnectionTracker 인터페이스 구현
- `DialTCP()` - 통합 연결 생성
- `BuildGETRequest()`, `BuildPOSTRequest()` - 요청 빌드
- `RecordLatency()`, `RecordTimeout()` - 메트릭스 기록

**활용 방법 (향후):**
```go
type MyStrategy struct {
    strategy.BaseStrategy  // 임베딩
    customField int
}

func (s *MyStrategy) Execute(ctx context.Context, target Target) error {
    s.IncrementConnections()
    defer s.DecrementConnections()
    // ...
}
```

---

### 3. 연결 로직 통합 (`internal/netutil/dialer.go`)

다양한 연결 방식을 단일 인터페이스로 통합했습니다.

**핵심 함수:**
```go
// 기본 TCP 연결 (Multi-IP 라운드로빈 지원)
func DialWithConfig(ctx, network, address, timeout, bindCfg) (net.Conn, error)

// TLS 연결
func DialTLSWithConfig(ctx, address, serverName, timeout, bindCfg) (net.Conn, error)

// 레거시 호환
func DialTCPWithBind(ctx, address, timeout, bindIP) (net.Conn, error)
```

**ConnectionFactory:**
```go
factory := netutil.NewConnectionFactory(bindIP)
conn, err := factory.Dial(ctx, "example.com:80")
client := factory.CreateHTTPClient(&connectionCounter)
```

---

### 4. Evasion 헤더 통합 (`internal/httpdata/headers.go`)

WAF 우회 헤더 생성 기능을 공통화했습니다.

**EvasionHeaderGenerator:**
```go
evasion := httpdata.NewEvasionHeaderGenerator(httpdata.EvasionLevelAggressive)
headers := evasion.GenerateEvasionHeaders()
// ["Sec-Fetch-Dest: document", "Sec-CH-UA: ...", ...]
```

**StealthHeaderSet:**
```go
stealth := httpdata.NewStealthHeaderSet(userAgent, evasionLevel)
headers := stealth.GenerateHeaders(host, path, contentType, contentLength)
```

**Evasion Level:**
| Level | 설명 | 포함 헤더 |
|-------|------|----------|
| 1 (Basic) | 기본 | 없음 |
| 2 (Normal) | Sec-Fetch-* | DNT, Upgrade-Insecure-Requests, Sec-Fetch-* |
| 3 (Aggressive) | Client Hints | + Sec-CH-UA-*, X-Request-ID, TE |

---

### 5. 백오프 로직 통합 (`internal/netutil/backoff.go`)

재시도 지연 계산을 공통화했습니다.

**Backoff 구조체:**
```go
backoff := netutil.DefaultBackoff()
delay := backoff.Next()  // 1초 → 2초 → 4초 (지수 증가)
backoff.Reset()
```

**유틸리티 함수:**
```go
// 단순 선형 백오프
delay := netutil.CalculateBackoff(consecutiveFailures)

// 지수 백오프
delay := netutil.CalculateExponentialBackoff(attempt, baseDelay, maxDelay)

// 랜덤 딜레이
delay := netutil.RandomDelay(min, max)
```

---

### 6. MetricsAware 인터페이스 확장

모든 slow* 전략에 MetricsAware 인터페이스를 추가했습니다.

| 전략 | MetricsAware | ConnectionTracker |
|------|--------------|-------------------|
| Slowloris | ✅ | ✅ |
| SlowlorisClassic | ✅ | ✅ |
| SlowPost | ✅ | ✅ |
| SlowRead | ✅ | ✅ |
| HTTPFlood | ✅ | ✅ |
| H2Flood | ✅ | ✅ |
| HeavyPayload | ✅ | ✅ |
| RUDY | ✅ | ✅ |

---

### 7. Strategy Factory 패턴 (`internal/strategy/factory.go`)

전략 생성 로직을 main.go에서 분리하여 Factory 패턴으로 구현했습니다.

**기본 사용법:**
```go
factory := strategy.NewStrategyFactory(&cfg.Strategy, bindIP)
strat := factory.Create()  // 설정된 Type으로 생성

// 또는 특정 타입 지정
strat := factory.CreateByType("slowloris")

// HTTP method 지정 (http-flood용)
strat := factory.CreateWithMethod("http-flood", "POST")
```

**유틸리티 함수:**
```go
// 사용 가능한 전략 목록
strategies := strategy.AvailableStrategies()
// [{Name: "normal", Description: "Standard HTTP requests..."}, ...]

// 전략 타입 검증
err := strategy.ValidateStrategyType("slowloris")

// 전략별 기본값 조회
defaults := strategy.StrategyDefaults("rudy")
// map[string]interface{}{"chunk-delay-min": 1s, ...}

// 전략 분류
isSlowAttack := strategy.IsSlowAttack("slowloris")    // true
isFloodAttack := strategy.IsFloodAttack("http-flood") // true

// 권장 세션 수 계산
target, rate := strategy.RecommendedSessions("slowloris", 100)

// 리소스 사용량 예측
estimate := strategy.EstimateResourceUsage("h2-flood", 1000, 60*time.Second)
```

**main.go 간소화:**
```go
// Before (40+ lines)
func createStrategy(cfg *config.Config) strategy.AttackStrategy {
    switch cfg.Strategy.Type {
    case "slowloris":
        return strategy.NewSlowlorisClassic(...)
    case "rudy":
        rudyCfg := strategy.RUDYConfig{...}
        return strategy.NewRUDY(rudyCfg, ...)
    // ... 10+ cases
    }
}

// After (6 lines)
func createStrategy(cfg *config.Config) strategy.AttackStrategy {
    factory := strategy.NewStrategyFactory(&cfg.Strategy, cfg.BindIP)
    if cfg.Strategy.Type == "http-flood" {
        return factory.CreateWithMethod("http-flood", cfg.Target.Method)
    }
    return factory.Create()
}
```

---

### 8. CLI 상수 통합 (`cmd/loadtest/main.go`)

CLI 플래그 기본값을 `config.*` 상수로 교체했습니다.

**변경 전:**
```go
flag.IntVar(&cfg.Performance.TargetSessions, "sessions", 100, "...")
flag.DurationVar(&cfg.Strategy.Timeout, "timeout", 10*time.Second, "...")
```

**변경 후:**
```go
flag.IntVar(&cfg.Performance.TargetSessions, "sessions", config.DefaultTargetSessions, "...")
flag.DurationVar(&cfg.Strategy.Timeout, "timeout", config.DefaultConnectTimeout, "...")
```

**적용된 상수:**
| 플래그 | 상수 |
|--------|------|
| `-sessions` | `config.DefaultTargetSessions` |
| `-rate` | `config.DefaultSessionsPerSec` |
| `-timeout` | `config.DefaultConnectTimeout` |
| `-keepalive` | `config.DefaultKeepAliveInterval` |
| `-content-length` | `config.DefaultContentLength` |
| `-read-size` | `config.DefaultReadSize` |
| `-window-size` | `config.DefaultWindowSize` |
| `-post-size` | `config.DefaultPostDataSize` |
| `-requests-per-conn` | `config.DefaultRequestsPerConn` |
| `-max-streams` | `config.DefaultMaxStreams` |
| `-burst-size` | `config.DefaultBurstSize` |
| `-payload-depth` | `config.DefaultPayloadDepth` |
| `-payload-size` | `config.DefaultPayloadSize` |
| `-chunk-delay-*` | `config.DefaultChunkDelay*` |
| `-chunk-size-*` | `config.DefaultChunkSize*` |
| `-max-req-per-session` | `config.DefaultMaxReqPerSession` |
| `-keepalive-timeout` | `config.DefaultKeepAliveTimeout` |
| `-session-lifetime` | `config.DefaultSessionLifetime` |
| `-send-buffer` | `config.DefaultSendBufferSize` |
| `-evasion-level` | `config.EvasionLevelNormal` |
| `-max-failures` | `config.DefaultMaxConsecutiveFailures` |
| `-pulse-high` | `config.DefaultPulseHighTime` |
| `-pulse-low` | `config.DefaultPulseLowTime` |
| `-pulse-ratio` | `config.DefaultPulseLowRatio` |

---

### 9. parseBindIPs 개선

IP 파싱 로직을 `strings.FieldsFunc`로 간소화했습니다.

**변경 전:**
```go
func parseBindIPs(s string) []string {
    var ips []string
    for _, ip := range strings.Split(s, ",") {
        ip = strings.TrimSpace(ip)
        if ip != "" {
            ips = append(ips, ip)
        }
    }
    return ips
}
```

**변경 후:**
```go
func parseBindIPs(s string) []string {
    return strings.FieldsFunc(s, func(c rune) bool {
        return c == ',' || c == ' ' || c == ';'
    })
}
```

**지원 구분자:** `,`, ` ` (공백), `;`

---

## 파일 구조

```
cmd/
└── loadtest/
    └── main.go         # [UPDATED] Factory 사용, 상수 적용

internal/
├── config/
│   ├── config.go       # 설정 구조체
│   └── constants.go    # [NEW] 상수 정의
├── strategy/
│   ├── interface.go    # 인터페이스 정의
│   ├── base.go         # [NEW] BaseStrategy
│   ├── factory.go      # [NEW] Strategy Factory
│   ├── slowloris.go    # [UPDATED] MetricsAware 추가
│   ├── slowloris_classic.go
│   ├── slow_post.go
│   ├── slow_read.go
│   └── ...
├── netutil/
│   ├── addr.go         # IP 풀 관리
│   ├── conn.go         # ManagedConn
│   ├── dialer.go       # [UPDATED] 통합 다이얼러
│   └── backoff.go      # [NEW] 백오프 로직
└── httpdata/
    ├── headers.go      # [UPDATED] Evasion 헤더 추가
    ├── useragent.go
    ├── formdata.go
    └── path.go
```

---

## 향후 개선 방향

### Phase 1: 전략 리팩토링 (단기)

1. **BaseStrategy 임베딩 적용**
   - 모든 전략에 `BaseStrategy` 구조체 임베딩
   - 중복 코드 제거 (activeConnections, metricsCallback 등)

   ```go
   // 현재
   type Slowloris struct {
       keepAliveInterval time.Duration
       connConfig        netutil.ConnConfig
       headerRandomizer  *httpdata.HeaderRandomizer
       activeConnections int64
       metricsCallback   MetricsCallback
   }

   // 개선 후
   type Slowloris struct {
       strategy.BaseStrategy
       keepAliveInterval time.Duration
   }
   ```

2. **RUDY 전략 분할**
   - `rudy.go` (876줄) → 여러 파일로 분할
   - `rudy_session.go` - 세션 관리
   - `rudy_stats.go` - 통계
   - `rudy_request.go` - 요청 빌드

3. **공통 헤더 생성 통합**
   - RUDY의 `buildStealthHeaders()` → `httpdata.StealthHeaderSet` 사용

### Phase 2: 구조 개선 (중기)

1. ~~**Strategy Factory 패턴 도입**~~ ✅ 완료
   - `internal/strategy/factory.go` 구현
   - `StrategyFactory`, `AvailableStrategies()`, `ValidateStrategyType()` 등

2. **설정 파일 지원**
   - YAML/JSON 설정 파일 로드
   - CLI 플래그와 설정 파일 병합

3. **테스트 커버리지 확대**
   - 각 공통 모듈 단위 테스트
   - 전략별 통합 테스트

### Phase 3: 고급 기능 (장기)

1. **플러그인 시스템**
   - 외부 전략 동적 로드
   - 커스텀 메트릭스 콜백

2. **분산 실행**
   - 여러 노드에서 병렬 실행
   - 중앙 메트릭스 수집

3. **실시간 모니터링**
   - WebSocket 기반 대시보드
   - Prometheus/Grafana 연동

---

## 마이그레이션 가이드

### 새 전략 추가 시

1. `BaseStrategy` 임베딩
2. `Name()` 메서드 구현
3. `Execute()` 메서드 구현
4. 필요시 `NewXXX()` 생성자에서 `NewBaseStrategy()` 호출

```go
type NewStrategy struct {
    strategy.BaseStrategy
    customConfig CustomConfig
}

func NewNewStrategy(cfg CustomConfig, bindIP string) *NewStrategy {
    return &NewStrategy{
        BaseStrategy: strategy.NewBaseStrategy(bindIP, false, false),
        customConfig: cfg,
    }
}

func (s *NewStrategy) Name() string {
    return "new-strategy"
}

func (s *NewStrategy) Execute(ctx context.Context, target Target) error {
    // BaseStrategy 메서드 활용
    conn, err := s.DialTCP(ctx, "tcp", address, timeout)
    // ...
}
```

### 상수 추가 시

1. `internal/config/constants.go`에 적절한 섹션에 추가
2. 관련 주석 작성
3. 기존 매직 넘버 교체

---

## 참고 사항

- 빌드 검증: `go build ./...`
- 테스트 실행: `go test ./...`
- 코드 포맷: `go fmt ./...`

---

*Last Updated: 2025-12*
