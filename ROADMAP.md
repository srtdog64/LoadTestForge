# LoadTestForge Roadmap

## Phase 1: Security Simulation Enhancement (보안 시뮬레이션 정밀도 향상)

### 1.1 TLS Fingerprint Randomization (JA3 Spoofing)

**Priority:** High  
**Complexity:** Medium  
**Library:** `github.com/refraction-networking/utls`

**Description:**  
TLS 핸드셰이크 단계에서 다양한 클라이언트 특성을 가장하여 지능형 방어 시스템의 봇 식별을 우회.

**Implementation:**
```go
type TLSProfile string

const (
    ProfileChrome    TLSProfile = "chrome"
    ProfileFirefox   TLSProfile = "firefox"
    ProfileSafari    TLSProfile = "safari"
    ProfileRandomize TLSProfile = "random"
)

type JA3Config struct {
    Profile       TLSProfile
    RandomizeJA3  bool
    CustomJA3Hash string
}
```

**Checklist:**
- [ ] utls 라이브러리 통합
- [ ] Chrome/Firefox/Safari Client Hello 프로필 구현
- [ ] JA3 해시 랜덤화 옵션
- [ ] `--tls-profile` 플래그 추가
- [ ] 기존 HTTPS 전략과 통합

---

### 1.2 HTTP Header Randomization

**Priority:** Medium  
**Complexity:** Low

**Description:**  
HTTP 헤더 순서 랜덤화 및 비표준(허용되는) 헤더 추가로 봇 탐지 회피.

**Implementation:**
```go
type HeaderRandomizer struct {
    ShuffleOrder     bool
    AddDecoyHeaders  bool
    VaryAcceptEncode bool
}

func (h *HeaderRandomizer) RandomizeHeaders(base http.Header) http.Header {
    // Shuffle header order
    // Add realistic decoy headers (X-Requested-With, DNT, etc.)
    // Vary Accept-Encoding combinations
}
```

**Checklist:**
- [ ] 헤더 순서 셔플링
- [ ] 합법적 비표준 헤더 추가 (X-Requested-With, DNT, Sec-Fetch-*)
- [ ] Accept-Encoding 조합 다양화
- [ ] 브라우저별 헤더 패턴 프로필

---

### 1.3 Scenario-Based Testing (시나리오 기반 테스트)

**Priority:** High  
**Complexity:** High

**Description:**  
단일 엔드포인트 반복 대신, 로그인→토큰획득→API호출 순의 시퀀스 실행.

**Implementation:**
```go
type ScenarioStep struct {
    Name         string
    Method       string
    Path         string
    Headers      map[string]string
    Body         string
    ExtractVars  map[string]string  // JSON path -> variable name
    Delay        time.Duration
}

type Scenario struct {
    Name   string
    Steps  []ScenarioStep
    Repeat int
}

// Example scenario
loginScenario := Scenario{
    Name: "Auth Flow",
    Steps: []ScenarioStep{
        {Name: "Login", Method: "POST", Path: "/login", ExtractVars: {"$.token": "auth_token"}},
        {Name: "Get Profile", Method: "GET", Path: "/profile", Headers: {"Authorization": "Bearer {{auth_token}}"}},
        {Name: "API Call", Method: "POST", Path: "/api/action"},
    },
}
```

**Checklist:**
- [ ] ScenarioStep / Scenario 구조체 정의
- [ ] JSON/YAML 시나리오 파일 파서
- [ ] 변수 추출 (JSON Path)
- [ ] 변수 치환 (템플릿)
- [ ] 세션별 독립 상태 유지
- [ ] `--scenario` 플래그 추가
- [ ] 샘플 시나리오 파일 제공

---

## Phase 2: Additional L7 Attack Vectors (L7 공격 확장)

### 2.1 HTTP/2 Slowloris

**Priority:** Medium  
**Complexity:** Medium

**Description:**  
HTTP/2 환경에서 단일 스트림의 헤더/DATA 프레임을 극도로 느리게 전송.

**Implementation:**
```go
type H2Slowloris struct {
    headerFrameDelay time.Duration  // HEADERS 프레임 간 지연
    dataFrameDelay   time.Duration  // DATA 프레임 간 지연
    frameSize        int            // 프레임당 바이트 수
}

// HEADERS 프레임을 여러 CONTINUATION으로 분할하여 천천히 전송
// DATA 프레임을 1바이트씩 전송
```

**Checklist:**
- [ ] HEADERS 프레임 분할 전송
- [ ] CONTINUATION 프레임 활용
- [ ] DATA 프레임 느린 전송
- [ ] 스트림 타임아웃 회피 로직

---

### 2.2 Cache Bypass Enhancement

**Priority:** Low  
**Complexity:** Low

**Description:**  
CDN 캐시 대량 소모를 위한 강화된 캐시 무력화 전략.

**Implementation:**
```go
type CacheBypass struct {
    RandomQueryParams   bool
    RandomPathSuffix    bool
    VaryCookies         bool
    RandomAcceptHeaders bool
    CacheBusterMethods  []string  // ["query", "path", "cookie", "header"]
}
```

**Checklist:**
- [ ] 정적 리소스 경로 변형 (/style.css?v=xxx)
- [ ] 쿠키 변조 (캐시 키에 쿠키 포함하는 서버 대상)
- [ ] Vary 헤더 악용
- [ ] Range 요청 분할

---

### 2.3 WebSocket Flood

**Priority:** Medium  
**Complexity:** Medium

**Description:**  
WebSocket 핸드셰이크 대량 전송 및 연결 유지 공격.

**Implementation:**
```go
type WebSocketFlood struct {
    upgradeOnly     bool  // 핸드셰이크만 반복
    keepAlive       bool  // 연결 유지 후 Ping/Pong
    messageFlood    bool  // 메시지 폭주
    messageSize     int
    pingInterval    time.Duration
}
```

**Checklist:**
- [ ] `gorilla/websocket` 라이브러리 통합
- [ ] Upgrade 요청 폭주 모드
- [ ] 연결 유지 + Ping 남용 모드
- [ ] 메시지 폭주 모드
- [ ] `--strategy websocket-flood` 추가

---

### 2.4 gRPC Stream Abuse

**Priority:** Low  
**Complexity:** High

**Description:**  
gRPC 스트리밍 연결 남용 공격.

**Implementation:**
```go
type GRPCFlood struct {
    unaryFlood       bool  // Unary 요청 폭주
    streamFlood      bool  // Bidirectional stream 남용
    streamKeepOpen   bool  // 스트림 열어두기
}
```

**Checklist:**
- [ ] gRPC 클라이언트 구현
- [ ] Unary 요청 폭주
- [ ] 양방향 스트림 남용
- [ ] 메타데이터 폭주

---

## Phase 3: Architecture Improvements (구조 개선)

### 3.1 TCP Socket Management Abstraction

**Priority:** High  
**Complexity:** Medium

**Description:**  
Slowloris, SlowPost, SlowRead 등에서 반복되는 소켓 관리 로직 통합.

**Implementation:**
```go
type ManagedConn struct {
    conn          net.Conn
    activeCounter *int64
    metrics       *metrics.Collector
    localAddr     *net.TCPAddr
    timeout       time.Duration
}

func NewManagedConn(target string, opts ConnOptions) (*ManagedConn, error) {
    // Dial, bind IP, set timeouts, increment counter
}

func (m *ManagedConn) Close() error {
    // Decrement counter, close conn
}
```

**Checklist:**
- [ ] ManagedConn 구조체 정의
- [ ] 공통 Dial 로직 추출
- [ ] atomic 카운터 통합
- [ ] 기존 전략 리팩토링

---

### 3.2 Event-Driven Architecture (gnet)

**Priority:** Low  
**Complexity:** Very High

**Description:**  
epoll 기반 event-loop로 10만+ 연결 효율적 처리.

**Library:** `github.com/panjf2000/gnet`

**Consideration:**
- 현재 goroutine 모델로 충분한 성능 (수만 연결)
- gnet 도입 시 전체 아키텍처 재설계 필요
- ROI 분석 후 결정

**Checklist:**
- [ ] gnet 프로토타입 구현
- [ ] 기존 goroutine 모델과 벤치마크 비교
- [ ] 메모리 사용량 비교
- [ ] 유지보수 복잡도 평가

---

### 3.3 Distributed Mode (분산 모드)

**Priority:** Medium  
**Complexity:** High

**Description:**  
여러 노드에서 조율된 공격 실행.

**Implementation:**
```go
type NodeRole string

const (
    RoleController NodeRole = "controller"
    RoleAgent      NodeRole = "agent"
)

type DistributedConfig struct {
    Role           NodeRole
    ControllerAddr string
    AgentAddrs     []string
    SyncInterval   time.Duration
}

// Controller: 전체 조율, 메트릭 수집
// Agent: 실제 공격 실행, 상태 보고
```

**Checklist:**
- [ ] Controller/Agent 역할 분리
- [ ] gRPC 기반 통신 프로토콜
- [ ] 동기화된 시작/중지
- [ ] 분산 메트릭 집계
- [ ] `--role`, `--controller` 플래그

---

### 3.4 Multi-Strategy Execution (다중 전략 동시 실행)

**Priority:** Medium  
**Complexity:** Medium

**Description:**  
여러 공격 벡터 동시 실행 (예: Slowloris 500 + HTTP Flood 500).

**Implementation:**
```go
type MultiStrategyConfig struct {
    Strategies []StrategyAllocation
}

type StrategyAllocation struct {
    Type     string
    Sessions int
    Weight   float64  // 또는 고정 세션 수
}

// Manager가 여러 Strategy를 관리
// 세션 풀을 전략별로 분배
```

**Checklist:**
- [ ] MultiStrategyManager 구현
- [ ] 전략별 세션 할당
- [ ] 전략별 메트릭 분리
- [ ] `--multi-strategy` 플래그 또는 설정 파일
- [ ] 전략 간 리소스 경합 관리

---

## Implementation Priority Matrix

| Feature | Impact | Effort | Priority |
|---------|--------|--------|----------|
| Scenario-Based Testing | High | High | P1 |
| TLS JA3 Spoofing | High | Medium | P1 |
| TCP Socket Abstraction | Medium | Medium | P1 |
| Multi-Strategy Execution | High | Medium | P2 |
| HTTP/2 Slowloris | Medium | Medium | P2 |
| WebSocket Flood | Medium | Medium | P2 |
| Distributed Mode | High | High | P2 |
| Header Randomization | Low | Low | P3 |
| Cache Bypass Enhancement | Low | Low | P3 |
| gRPC Flood | Low | High | P3 |
| Event-Driven (gnet) | Medium | Very High | P4 |

---

## Version Targets

### v1.1 (Short-term)
- TCP Socket Management Abstraction
- TLS JA3 Spoofing (basic)
- Header Randomization

### v1.2 (Medium-term)
- Scenario-Based Testing
- Multi-Strategy Execution
- HTTP/2 Slowloris

### v2.0 (Long-term)
- Distributed Mode
- WebSocket Flood
- gRPC Flood
- Full scenario orchestration
