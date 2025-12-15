# LoadTestForge 아키텍처

**최종 업데이트:** 2025-12-16 (KST)

## 1. 개요 (High-Level Overview)

LoadTestForge는 Go 언어로 작성된 모듈식 고성능 HTTP 부하 테스트 및 네트워크 스트레스 도구입니다. 실제 사용자 행동을 모방하거나 서버 및 네트워크 인프라의 복원력을 테스트하기 위해 다양한 공격 패턴(Flood, Slowloris, RUDY, HULK)을 시뮬레이션할 수 있도록 설계되었습니다.

## 2. 핵심 컴포넌트 (Core Components)

### 2.1. 커맨드 라인 인터페이스 (CMD)
`cmd/loadtest/main.go`에 위치한 진입점(Entry point)은 다음을 처리합니다:
- 플래그 파싱 및 설정 유효성 검사.
- 핵심 컴포넌트(Metrics, Session Manager) 초기화.
- 사용자 안전 확인 (Target 유효성 검사, Public IP 경고).
- 적절한 종료를 위한 시그널 핸들링.
- **난수 시드 초기화** 및 **IP 바인딩 범위 파싱** (New).

### 2.2. 세션 관리 (Session Management)
**Session Manager** (`internal/session/manager.go`)는 부하 생성을 조정합니다:
- 가상 "세션"(사용자/연결)의 수명 주기를 관리합니다.
- `rate.Limiter`를 사용하여 동시성을 제어하고 설정된 속도(`SessionsPerSec`)를 준수합니다.
- "Pulsing" 부하 패턴(Square, Sine, Sawtooth 파형) 구현.
- 램프업(Ramp-up) 및 정상 상태(Steady-state) 단계 처리.

### 2.3. 공격 전략 (Attack Strategies)
전략들은 `AttackStrategy` 인터페이스(`internal/strategy`)를 구현합니다. 팩토리 패턴(`factory.go`)을 사용하여 인스턴스화됩니다.

**지원되는 전략:**
- **Normal/KeepAlive**: 표준 HTTP 벤치마킹.
- **Slowloris**: 느린 헤더 고갈 공격 (Classic & Keep-Alive 변형).
- **RUDY (R-U-Dead-Yet)**: 정교한 회피 기술이 적용된 느린 POST 바디 고갈 공격.
- **HTTP Flood**: 고성능 GET/POST 플러딩.
- **H2 Flood**: HTTP/2 멀티플렉싱 플러딩.
- **TCP Flood**: Raw TCP 연결 고갈.
- **HULK (New)**: 강화된 HTTP Unbearable Load King.

#### 2.3.1. 강화된 HULK 전략
Python HULK 스크립트에서 영감을 받은 Go 구현체 특징:
- **동적 파라미터 주입**: 캐싱을 우회하기 위해 쿼리 파라미터를 무작위화.
- **스마트 회피**: 선별된 목록(`internal/httpdata`)에서 User-Agent 및 Referer를 순환.
- **WAF 우회**: Stealth 헤더(`Sec-Fetch-*`, `Upgrade-Insecure-Requests`) 주입.
- **연결 추적**: 활성 연결을 모니터링하고 CPS(Connections Per Second)를 보고.
- **공통 모듈 활용**: `httpdata` 패키지의 공통 로직(Junk 파라미터, 헤더 생성기)을 사용하여 코드 중복 최소화.

### 2.4. 메트릭 시스템 (Metrics System)
메트릭 시스템(`internal/metrics`)은 실시간 가시성을 제공합니다:
- **Collector**: `sync/atomic` 및 `sync.Mutex`를 사용한 스레드 안전한 카운터 집계.
  - RPS (Requests Per Second) 및 **CPS (Connections Per Second)** 추적.
  - 메모리 누수 방지를 위한 슬라이딩 윈도우 데이터 관리 (최근 1시간 데이터 유지).
  - 레이턴시(p50, p95, p99), 성공률, 오류 유형 모니터링.
- **Reporter**: 주기적으로 포맷된 통계를 콘솔에 출력하고 최종 요약 보고서를 생성.

### 2.5. 네트워크 유틸리티 (Netutil)
`internal/netutil`은 저수준 네트워크 제어를 제공합니다:
- **Custom Dialer**: 타임아웃, keep-alive, 소스 IP 바인딩(단일/다중/범위)에 대한 정밀 제어.
- **TrackedTransport**: 활성 TCP 연결을 추적하는 특수 `http.Transport`.
- **Metrics Hooks**: 연결 시도 및 소켓 이벤트를 기록하기 위한 콜백.

## 3. 데이터 흐름 (Data Flow)

1. **초기화**: Config -> Factory -> Strategy Instance.
2. **실행**: Manager가 N개의 고루틴(Session)을 생성 (Rate Limiter 적용).
3. **루프**: 각 세션은 `strategy.Execute(ctx, target)`을 호출.
4. **네트워크**: 전략은 `netutil`을 사용하여 Dial/Request 수행.
   - `OnDial` 훅이 `metrics.RecordConnectionAttempt` 트리거.
5. **텔레메트리**: 성공/실패/레이턴시가 `metrics.Collector`에 기록.
6. **보고**: `metrics.Reporter`가 Collector를 읽어 CLI에 출력.

## 4. 주요 설계 패턴 (Key Design Patterns)
- **Interface-based Strategy**: 새로운 공격 벡터의 확장을 용이하게 함.
- **Worker Pools**: 대규모 동시성을 위한 효율적인 고루틴 관리.
- **Context Propagation**: `context.Context`를 통한 취소 및 타임아웃 처리.
- **Atomic Counters**: 성능을 위한 락-프리(Lock-free) 메트릭 업데이트.
