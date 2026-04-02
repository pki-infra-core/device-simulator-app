# device-simulator-app

`device-simulator-app` 은 `device-identity-sdk` 통합을 검증하기 위한 standalone Go CLI 다

이 프로젝트는 실제 device business logic 을 구현하지 않는다  
대신 가상 device 처럼 동작하면서 bootstrap, local persistence, identity reload, mTLS API 호출, fake telemetry, reset, renew 흐름을 재현한다

## 목적

- `device-identity-sdk` 를 실제 애플리케이션처럼 소비
- 로컬 key 생성과 CSR 생성이 SDK 내부에 머무는지 확인
- `device-cert-issuer` 와의 bootstrap / renewal 연동 검증
- 재시작 이후 저장된 identity 를 다시 로드하는 흐름 검증
- 보호된 mTLS API 접근 성공 여부 확인
- fake telemetry 전송 경로 검증

## 주요 가정

- `device-identity-sdk` 는 인접 경로 `../device-identity-sdk` 에 존재한다
- simulator 는 SDK 의 file store 를 사용한다
- SDK 가 encrypted private key 복호화에 passphrase 를 요구하므로 `DEVICE_IDENTITY_PASSPHRASE` 를 환경 변수로 받는다
- reset 은 SDK 저장소 디렉터리 전체를 제거해서 clean re-bootstrap 을 가능하게 한다

## 아키텍처

- `cmd/device-simulator`
  - CLI entrypoint
- `internal/config`
  - 환경 변수와 subcommand flag 로딩
- `internal/runtime`
  - SDK, store, issuer client, gateway client wiring
- `internal/app`
  - bootstrap, status, ping, telemetry, renew, reset command orchestration
- `internal/gateway`
  - SDK 가 만든 mTLS `tls.Config` 를 사용하는 protected API client
- `internal/output`
  - 콘솔 출력 정리

## 디렉터리 구조

```text
device-simulator-app/
├── Makefile
├── README.md
├── go.mod
├── cmd/
│   └── device-simulator/
│       └── main.go
├── examples/
│   └── .env.example
└── internal/
    ├── app/
    │   ├── app.go
    │   └── app_test.go
    ├── config/
    │   ├── config.go
    │   └── config_test.go
    ├── gateway/
    │   ├── client.go
    │   └── client_test.go
    ├── output/
    │   ├── output.go
    │   └── output_test.go
    └── runtime/
        ├── issuer.go
        └── runtime.go
```

## 전체 흐름

이 흐름은 두 단계로 나뉜다

### 1. Bootstrap

- gateway 가 local 에서 server private key 와 CSR 을 생성한다
- CSR 만 `device-cert-issuer` 로 보내고, signed server certificate 를 돌려받아 저장한다
- `device-simulator-app` 은 이 과정을 `device-identity-sdk` 의 `Bootstrap(...)` 호출로 수행한다
- 앱은 private key PEM 을 직접 만들거나 파싱하지 않는다

### 2. Runtime

- `device-simulator-app` 은 `GATEWAY_CA_FILE` 로 gateway server certificate 를 검증한다
- `mock-device-gateway` 는 `CLIENT_ROOT_CA_PATH` 로 device client certificate chain 을 검증한다
- 양쪽 검증이 모두 성공하면 `/api/v1/ping` 이 정상 응답한다
- `telemetry` 도 같은 mTLS 세션 구성 원칙으로 보호된 endpoint 를 호출한다

## CLI 설계

```bash
device-simulator <command> [flags]
```

지원 명령

- `bootstrap`
  - 새 device identity 발급 및 저장
  - `--force` 로 기존 저장소 제거 후 재등록 가능
- `status`
  - 저장된 identity 조회와 상태 판정
- `ping`
  - `GET /api/v1/ping` 호출
- `telemetry`
  - `POST /api/v1/telemetry` 호출
- `renew`
  - identity renewal 수행
  - `--rotate-key` 로 key rotation 테스트 가능
- `reset`
  - 로컬 identity 저장소 제거

## 환경 변수

필수

- `DEVICE_ID`
- `TENANT_ID`
- `DEVICE_MODEL`
- `IDENTITY_STORAGE_DIR`
- `DEVICE_IDENTITY_PASSPHRASE`

bootstrap / renew 에 필요

- `ISSUER_BASE_URL`
- `BOOTSTRAP_TOKEN`

ping / telemetry 에 필요

- `GATEWAY_BASE_URL`

선택

- `DEVICE_PROFILE`
- `HTTP_TIMEOUT`
- `LOG_LEVEL`
- `FORCE_BOOTSTRAP`
- `RENEW_BEFORE`
- `GATEWAY_CA_FILE`
- `GATEWAY_SERVER_NAME`
- `PING_PATH`
- `TELEMETRY_PATH`
- `TELEMETRY_TEMPERATURE`
- `TELEMETRY_BATTERY_PCT`
- `FIRMWARE_VERSION`
- `TELEMETRY_STATUS`

## Bootstrap 흐름

1. CLI 가 config 를 읽는다
2. runtime 이 `device-identity-sdk` 와 file store 를 구성한다
3. app 계층이 `sdk.Bootstrap(...)` 을 호출한다
4. SDK 가 key 생성, CSR 생성, issuer 호출, 응답 검증, 저장을 수행한다
5. simulator 는 저장된 identity summary 만 출력한다

앱은 private key PEM 을 직접 만들거나 파싱하지 않는다  
mTLS 용 `tls.Config` 도 SDK public API 로만 받는다

## Status 흐름

`status` 는 `sdk.LoadIdentity(...)` 와 `sdk.ValidateIdentity(...)` 를 사용해 다음 상태를 보여 준다

- `READY`
- `INVALID`
- `NOT_FOUND`
- `EXPIRING_SOON`

## Protected API 호출

`ping` 과 `telemetry` 는 모두 다음 절차를 따른다

1. SDK 로 client `tls.Config` 생성
2. `GATEWAY_CA_FILE` 을 사용해서 server certificate trust anchor 를 구성한다
3. `net/http` client 구성
4. gateway endpoint 호출
5. `mock-device-gateway` 가 `CLIENT_ROOT_CA_PATH` 로 device client certificate chain 을 검증한다
6. HTTP 결과와 응답 body 요약 출력

## 예시 사용 순서

1. 환경 변수 설정
2. `make bootstrap`
3. `make status`
4. `make ping`
5. `make telemetry`
6. `make renew`
7. `make reset`

## 로컬 사용 예시

```bash
cp examples/.env.example .env
export $$(grep -v '^#' .env | xargs)

make bootstrap
make status
make ping
make telemetry
make renew
make reset
```

특정 옵션 예시

```bash
go run ./cmd/device-simulator bootstrap --force
go run ./cmd/device-simulator renew --rotate-key
```

## 이 프로젝트가 SDK 검증에 주는 가치

- SDK consumer 가 따라야 할 최소 wiring 예시 제공
- bootstrap 부터 mTLS 호출까지 end-to-end smoke path 제공
- renew / reset 테스트를 위한 명확한 CLI entrypoint 제공
- private key lifecycle 을 앱 바깥으로 새지 않게 유지하는 사용 패턴 제공

## 테스트

```bash
make test
```

포함 범위

- config loading
- bootstrap command flow
- status command behavior
- ping / telemetry gateway behavior
- output formatting helper
