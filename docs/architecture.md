# LoadTestForge 아키텍처

**최종 업데이트:** 2025-12-16 (KST)

## 1. 개요 (High-Level Overview)

LoadTestForge는 Go 언어로 작성된 모듈식 고성능 HTTP 부하 테스트 및 네트워크 스트레스 도구입니다. 실제 사용자 행동을 모방하거나 서버 및 네트워크 인프라의 복원력을 테스트하기 위해 다양한 공격 패턴(Flood, Slowloris, RUDY, HULK)을 시뮬레이션할 수 있도록 설계되었습니다.

```
┌─────────────────────────────────────────────────────────────┐
│                    CLI Entry Point (main.go)                │
│  - Flag parsing, config validation, graceful shutdown       │
├─────────────────────────────────────────────────────────────┤
│                    Session Manager                          │
│  - Session lifecycle (spawn/prune)                         │
│  - Rate limiting (token bucket)                            │
│  - Pulse mode (sine/sawtooth/square)                       │
├─────────────────────────────────────────────────────────────┤
│              Attack Strategy System (Strategy Pattern)       │
│  - 12 strategies (normal, keepalive, slowloris, etc.)      │
│  - BaseStrategy (common functionality)                      │
│  - MetricsAware interface                                   │
├─────────────────────────────────────────────────────────────┤
│               Network Utilities (netutil package)           │
│  - Multi-IP round-robin binding                             │
│  - Connection pooling and tracking                          │
│  - OnDial hooks for CPS tracking                           │
│  - Configurable TLS settings                                │
├─────────────────────────────────────────────────────────────┤
│              Random Utilities (randutil package)            │
│  - Per-goroutine pooled rand (high CPS optimization)       │
│  - Lock-free random number generation                       │
├─────────────────────────────────────────────────────────────┤
│              Error Classification (errors package)          │
│  - Network/Timeout/TLS/Protocol/HTTP error types           │
│  - ClassifyAndWrap for consistent error handling           │
│  - Retry decision support                                   │
├─────────────────────────────────────────────────────────────┤
│                  Metrics Collection                         │
│  - Real-time statistics aggregation                         │
│  - Percentile analysis (p50, p95, p99)                     │
│  - Configurable pass/fail thresholds                       │
└─────────────────────────────────────────────────────────────┘
```

## 2. 핵심 컴포넌트 (Core Components)

### 2.1. 커맨드 라인 인터페이스 (CMD)
`cmd/loadtest/main.go`에 위치한 진입점(Entry point)은 다음을 처리합니다:
- 플래그 파싱 및 설정 유효성 검사 (30+ 옵션).
- 핵심 컴포넌트(Metrics, Session Manager) 초기화.
- 사용자 안전 확인 (Target 유효성 검사, Public IP 경고).
- 적절한 종료를 위한 시그널 핸들링.
- **IP 바인딩 범위 파싱** (예: `192.168.1.10-20`).
- **IP 범위 안전 제한**: `MaxIPsPerRange=256`, `MaxTotalBindIPs=1024`.
- **Configurable pass/fail thresholds** 설정.

### 2.2. 설정 시스템 (Configuration)
`internal/config` 패키지가 모든 설정을 관리합니다:
- **StrategyConfig**: 전략별 설정 (타임아웃, 페이로드, TLS 등).
- **PerformanceConfig**: 세션 수, 속도, 램프업, 펄스 모드.
- **ThresholdsConfig**: Pass/Fail 판정 기준 (성공률, 레이턴시, 타임아웃률).
- **검증**: Payload depth/size 제한, Pulse ratio 범위, Threshold 범위.
- **상수화된 타임아웃**: `DefaultDialerTimeout`, `DefaultStreamTimeout` 등.

### 2.3. 세션 관리 (Session Management)
**Session Manager** (`internal/session/manager.go`)는 부하 생성을 조정합니다:
- 가상 "세션"(사용자/연결)의 수명 주기를 관리합니다.
- `rate.Limiter`를 사용하여 동시성을 제어하고 설정된 속도(`SessionsPerSec`)를 준수합니다.
- "Pulsing" 부하 패턴(Square, Sine, Sawtooth 파형) 구현.
- 램프업(Ramp-up) 및 정상 상태(Steady-state) 단계 처리.
- **연속 실패 감지**: `MaxConsecutiveFailures` 초과 시 세션 종료.
- **CPU 스핀 방지**: Rate limiter blocking으로 효율적인 세션 스폰.

### 2.4. 공격 전략 (Attack Strategies)
전략들은 `AttackStrategy` 인터페이스(`internal/strategy`)를 구현합니다. 팩토리 패턴(`factory.go`)을 사용하여 인스턴스화됩니다.

**인터페이스 계층:**
```go
type AttackStrategy interface {
    Execute(ctx context.Context, target Target) error
    Name() string
}

type MetricsAware interface {
    SetMetricsCallback(callback MetricsCallback)
}

type ConnectionTracker interface {
    ActiveConnections() int64
}

type SelfReportingStrategy interface {
    IsSelfReporting() bool
}
```

**지원되는 전략 (12종):**

| 전략 | 유형 | 설명 |
|------|------|------|
| **normal** | Stateless | 요청당 하나의 연결 |
| **keepalive** | Persistent | Keep-alive로 연결 재사용 |
| **slowloris** | Slow | 불완전한 HTTP 헤더 전송 |
| **slowloris-classic** | Slow | 클래식 Slowloris 구현 |
| **slow-post** | Slow | 느린 POST 바디 전송 |
| **slow-read** | Slow | 느린 응답 읽기 |
| **http-flood** | Flood | 대량 HTTP 요청 |
| **h2-flood** | Flood | HTTP/2 스트림 플러딩 |
| **heavy-payload** | CPU | CPU 집약적 페이로드 (JSON/XML/ReDoS) |
| **hulk** | Hybrid | 강화된 HULK 공격 |
| **rudy** | Slow | R.U.D.Y. 고급 slow POST |
| **tcp-flood** | Connection | TCP 연결 고갈 |

#### 2.4.1. BaseStrategy 공통 기능
모든 전략이 상속하는 공통 기능:
- **연결 추적**: Atomic 카운터로 활성 연결 수 관리.
- **Multi-IP 바인딩**: Round-robin IP 풀 지원.
- **메트릭 콜백**: 레이턴시, 타임아웃, 재연결 기록.
- **OnDial 훅**: CPS 추적을 위한 자동 연결 시도 기록.
- **헤더 랜덤화**: User-Agent, Referer, Sec-Fetch-* 헤더.
- **TLS 설정**: `TLSSkipVerify` 옵션으로 인증서 검증 제어.

### 2.5. 에러 분류 시스템 (Error Classification)
`internal/errors` 패키지가 에러를 분류합니다:
- **ErrorType**: Network, Timeout, HTTP, TLS, Protocol, Canceled.
- **Classify()**: 에러 문자열 분석으로 자동 분류.
- **ClassifyAndWrap()**: 에러 분류 + 컨텍스트 래핑 (모든 전략에서 사용).
- **IsRetryable()**: 재시도 가능 여부 판단.
- **HTTPError**: HTTP 상태 코드 기반 에러 (4xx/5xx 구분).
- **ErrorStats**: 유형별 에러 통계 추적.

```go
// 모든 전략에서 일관된 에러 처리
if err != nil {
    return errors.ClassifyAndWrap(err, "connection failed")
}
```

### 2.6. 랜덤 유틸리티 (Random Utilities)
`internal/randutil` 패키지는 고성능 난수 생성을 제공합니다:
- **Pool-based Rand**: `sync.Pool`로 고루틴별 `*rand.Rand` 인스턴스 관리.
- **Lock Contention 제거**: 전역 rand 뮤텍스 병목 해소.
- **편의 함수**: `Intn()`, `Float32()`, `Perm()`, `Shuffle()` 등.

```go
// 고성능 시나리오에서 사용
rng := randutil.Get()
defer rng.Release()
value := rng.Intn(1000000)
```

### 2.7. 메트릭 시스템 (Metrics System)
메트릭 시스템(`internal/metrics`)은 실시간 가시성을 제공합니다:
- **Collector**: `sync/atomic` 및 `sync.Mutex`를 사용한 스레드 안전한 카운터 집계.
  - RPS (Requests Per Second) 및 **CPS (Connections Per Second)** 추적.
  - 메모리 누수 방지를 위한 슬라이딩 윈도우 데이터 관리 (최근 1시간 데이터 유지).
  - 레이턴시(p50, p95, p99), 성공률, 오류 유형 모니터링.
- **Reporter**: 주기적으로 포맷된 통계를 콘솔에 출력하고 최종 요약 보고서를 생성.
- **Configurable Thresholds**: Pass/Fail 판정 기준을 CLI에서 설정 가능.
  - `-min-success-rate`: 최소 성공률 (기본: 90%)
  - `-max-rate-deviation`: 최대 요청률 편차 (기본: 20%)
  - `-max-p99-latency`: 최대 p99 레이턴시 (기본: 5s)
  - `-max-timeout-rate`: 최대 타임아웃률 (기본: 10%)

### 2.8. 네트워크 유틸리티 (Netutil)
`internal/netutil`은 저수준 네트워크 제어를 제공합니다:
- **Custom Dialer**: 타임아웃, keep-alive, 소스 IP 바인딩(단일/다중/범위)에 대한 정밀 제어.
- **TrackedTransport**: 활성 TCP 연결을 추적하는 특수 `http.Transport`.
- **ManagedConn**: 자동 카운터 관리 및 세션 수명 제어.
- **ConnConfig.OnDial**: 모든 연결 시도에서 CPS 추적 훅 지원.
- **Configurable TLS**: `TLSSkipVerify` 옵션으로 인증서 검증 제어.
- **Metrics Hooks**: 연결 시도 및 소켓 이벤트를 기록하기 위한 콜백.

## 3. 데이터 흐름 (Data Flow)

```
┌──────────┐    ┌─────────┐    ┌──────────┐    ┌─────────┐
│  Config  │───▶│ Factory │───▶│ Strategy │───▶│ Execute │
└──────────┘    └─────────┘    └──────────┘    └────┬────┘
                                                     │
     ┌───────────────────────────────────────────────┘
     ▼
┌──────────┐    ┌─────────┐    ┌──────────┐    ┌─────────┐
│  Netutil │───▶│  Dial   │───▶│ Request  │───▶│Response │
│ (OnDial) │    └─────────┘    └──────────┘    └────┬────┘
└──────────┘                                         │
     ┌───────────────────────────────────────────────┘
     ▼
┌──────────┐    ┌─────────┐    ┌──────────┐
│ Classify │───▶│Collector│───▶│ Reporter │
│ AndWrap  │    └─────────┘    └──────────┘
└──────────┘
```

1. **초기화**: Config -> Factory -> Strategy Instance.
2. **실행**: Manager가 N개의 고루틴(Session)을 생성 (Rate Limiter 적용).
3. **루프**: 각 세션은 `strategy.Execute(ctx, target)`을 호출.
4. **네트워크**: 전략은 `netutil`을 사용하여 Dial/Request 수행.
   - `OnDial` 훅이 `metrics.RecordConnectionAttempt` 트리거 (모든 전략).
5. **에러 분류**: 실패 시 `errors.ClassifyAndWrap()`으로 에러 유형 분류 및 래핑.
6. **텔레메트리**: 성공/실패/레이턴시가 `metrics.Collector`에 기록.
7. **보고**: `metrics.Reporter`가 Collector를 읽어 CLI에 출력.
8. **판정**: 설정된 threshold 기준으로 Pass/Fail 결정.

## 4. 주요 설계 패턴 (Key Design Patterns)
- **Interface-based Strategy**: 새로운 공격 벡터의 확장을 용이하게 함.
- **Composition over Inheritance**: `BaseStrategy` 임베딩으로 코드 재사용.
- **Factory Pattern**: 전략 인스턴스 생성 로직 캡슐화.
- **Worker Pools**: 대규모 동시성을 위한 효율적인 고루틴 관리.
- **Context Propagation**: `context.Context`를 통한 취소 및 타임아웃 처리.
- **Atomic Counters**: 성능을 위한 락-프리(Lock-free) 메트릭 업데이트.
- **Error Classification**: 에러 유형별 처리 및 재시도 결정.
- **Object Pool**: `randutil`의 `sync.Pool`로 고성능 난수 생성.

## 5. 패키지 구조 (Package Structure)

```
LoadTestForge/
├── cmd/loadtest/          # CLI 진입점
│   └── main.go
├── internal/
│   ├── config/            # 설정 구조체 및 상수
│   │   ├── config.go
│   │   └── constants.go
│   ├── errors/            # 에러 분류 시스템
│   │   ├── errors.go
│   │   └── errors_test.go
│   ├── httpdata/          # HTTP 데이터 (User-Agent, Headers)
│   ├── metrics/           # 메트릭 수집 및 보고
│   │   ├── collector.go
│   │   └── reporter.go
│   ├── netutil/           # 네트워크 유틸리티
│   │   ├── addr.go        # IP 풀 관리
│   │   ├── conn.go        # 관리형 연결 (OnDial 훅 포함)
│   │   └── dialer.go      # 커스텀 다이얼러
│   ├── randutil/          # 고성능 난수 생성
│   │   ├── rand.go        # Pool-based rand
│   │   └── rand_test.go
│   ├── session/           # 세션 관리
│   │   └── manager.go
│   └── strategy/          # 공격 전략
│       ├── interface.go   # 인터페이스 정의
│       ├── base.go        # 공통 기능 (OnDial 자동 연결)
│       ├── factory.go     # 팩토리 패턴
│       └── *.go           # 12개 전략 구현
└── docs/
    └── architecture.md
```

## 6. 성능 최적화 (Performance Optimizations)

### 6.1. 고성능 난수 생성
- 전역 `math/rand` 뮤텍스 병목 해소를 위해 `randutil` 패키지 도입.
- `http-flood`, `h2-flood` 등 고CPS 전략에서 활용.

### 6.2. Rate Limiter 기반 스폰
- Steady-state에서 CPU 스핀 방지를 위해 `rate.Limiter.Wait()` 직접 사용.
- 세션당 블로킹으로 효율적인 리소스 사용.

### 6.3. OnDial 훅 통합
- 모든 전략(HTTP, Slow, TCP)에서 일관된 CPS 추적.
- `BaseStrategy.GetConnConfig()`에서 자동으로 OnDial 훅 연결.

### 6.4. IP 범위 안전 제한
- `MaxIPsPerRange=256`: 단일 범위당 최대 IP 수.
- `MaxTotalBindIPs=1024`: 전체 바인딩 IP 최대 수.

## 7. CLI 사용 예시

```bash
# 기본 HTTP Flood 테스트
./loadtest -target https://example.com -strategy http-flood -sessions 100

# Slowloris with custom thresholds
./loadtest -target https://example.com -strategy slowloris \
  -sessions 500 -duration 5m \
  -min-success-rate 80 -max-timeout-rate 20

# Multi-IP binding with ramp-up
./loadtest -target https://example.com -strategy keepalive \
  -bind-ip "192.168.1.10-20" -sessions 1000 -rampup 30s

# Heavy payload with TLS verification disabled
./loadtest -target https://example.com -strategy heavy-payload \
  -payload-type deep-json -payload-depth 100 \
  -tls-skip-verify=true

# HTTP/2 flood with high concurrency
./loadtest -target https://example.com -strategy h2-flood \
  -sessions 50 -max-streams 100 -burst-size 10
```
