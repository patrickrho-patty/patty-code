# Patty Code 확장(Extensions)

확장(Extensions)을 사용하면 플러그인 패키지가 런타임에서 Patty Code의 동작을 변경할 수 있습니다. 입력 재작성, 도구 호출 가로채기, 시스템 프롬프트 교체, 스트리밍 모델 제공자 추가, 구조화된 UI 게시, 프롬프트와 테마 제공 등을 안정적이고 버전이 관리되는 계약(contract)을 통해 수행할 수 있습니다.

플러그인 기능에는 두 가지 종류가 있습니다:

- **선언형(Declarative)** (모든 플러그인 패키지): 스킬(skills), 에이전트(agents), 명령(commands), 프롬프트(prompts), 훅(hooks), MCP 서버, 테마(themes). 이들은 파일과 구성으로 이루어져 있으며 호스트의 일반 권한으로 실행됩니다.
- **코드 런타임(Code runtime)** (Manifest v2 `runtime` 블록): Extension Protocol을 사용하는 사이드카(sidecar) 프로세스입니다. 코드 확장은 **완전 신뢰(full trust)** 를 요구하므로, 설치하기 전에 아래의 보안 섹션을 반드시 확인하세요.

## 설치 및 관리

확장은 다른 플러그인 패키지와 완전히 동일한 방식으로 설치됩니다:

```bash
patcode plugin install git:github.com/owner/extension --dry-run   # preview
patcode plugin install git:github.com/owner/extension --yes       # install
patcode plugin show <name>                                        # details
patcode plugin doctor <name>                                      # validate
```

`runtime` 블록이 있는 플러그인의 경우 미리보기와 `show` 출력에 **FULL TRUST** 블록이 포함됩니다. 런타임 명령, 가로채는 이벤트, 소유한 교체 슬롯, 제공자/UI 기능이 표시됩니다. 설치, 업데이트, 교체 또는 `--link`가 곧 승인(authorization)이며 별도의 재확인 절차는 없습니다. `--link`는 변경된 콘텐츠도 계속 신뢰합니다. 완전히 신뢰하는 런타임만 설치하세요.

## 확장이 할 수 있는 작업

- **인터셉터(Interceptors)** — 입력, 도구 호출, 권한 결정, 제공자 요청/응답, 압축(compaction), 세션 수명 주기, 프론트엔드 이벤트 등 17개의 훅 지점을 관찰하고 판정합니다. 인터셉터는 `continue`, 사용자에게 보이는 사유를 동반한 `block`, 또는 페이로드 `replace`를 수행할 수 있으며, 호스트는 모든 교체를 다시 검증합니다.
- **교체 전략(Replacement strategies)** — 단일 소유자 슬롯(`system_prompt`, `context`, `provider_request`, `provider_response`, `compaction`, `session_policy`, `permission`, `frontend_events`, `tool:<name>`, `provider:<ref>`). 설치된 모든 플러그인에서 슬롯당 소유자는 하나뿐이며, 충돌이 발생하면 두 소스를 모두 명시한 채 런타임 빌드가 실패합니다.
- **스트리밍 제공자(Streaming providers)** — 새 모델이 모델 선택기에서 `plugin/<plugin>/<provider>/<model>`로 나타나며, 기본 제공자와 동일한 text/reasoning/tool-call/usage 시맨틱으로 스트리밍됩니다. 이 ref는 기본 제공자의 ref가 작동하는 모든 곳에서 작동합니다. `default_model`, `--model`, CLI/Desktop/ACP 선택기, 세션 중간 모델 전환 — 최초 부팅 시에도 포함됩니다.
- **구조화된 UI(Structured UI)** — 상태 항목, 카드, 폼, 알림이 CLI 트랜스크립트, Desktop 앱, ACP 클라이언트에서 네이티브로 렌더링되며(텍스트 폴백 포함), 슬래시 메뉴, Desktop 명령 팔레트, ACP의 검색 가능한 명령에는 `/<plugin>:<action>` 액션도 추가됩니다.
- **프롬프트와 테마(Prompts and themes)** — `/<plugin>:<name>` 프롬프트 템플릿과 Desktop 설정의 읽기 전용 플러그인 테마(`plugin:<plugin>:<theme>`).

## 런타임 리로드

설치된 확장을 변경해도(설치, 업데이트, 활성화/비활성화, `--link` 콘텐츠 변경) 실행 중인 턴(turn)은 절대 변경되지 않습니다. 리로드는 모든 대화형 프론트엔드에서 하나의 실패 원자적(fail-atomic) 작업으로 수행됩니다 — CLI `/reload`, Desktop **Reload Runtime**(명령 팔레트), Serve `/reload`, ACP 공급업체 메서드 `_patty-code.io/session/reloadExtensions`:

1. 턴 또는 백그라운드 작업이 실행 중이면 CLI/Desktop/ACP는 정확히 한 번의 리로드를 대기열에 넣습니다. Serve는 요청을 거부하므로 브라우저는 유휴 상태가 되면 다시 시도할 수 있습니다.
2. 유휴 상태가 되면 Patty Code는 새 사이드카를 시작하고 새 런타임 스냅샷을 빌드합니다.
3. 완전히 성공하면 원자적으로 교체하며 세션 경로, 트랜스크립트, 승인 권한(approval grants), 목표/복구 상태를 유지합니다.
4. 새 빌드가 실패하면 기존 런타임은 그대로 계속 작동합니다.
5. 교체가 끝난 후에만 이전 사이드카가 종료됩니다.

각 턴은 턴 전체, 도구 배치, 압축(compaction) 동안 하나의 런타임 세대(generation)를 고정합니다. 확장 변경 사항은 *다음* 턴부터 적용되며, 변경 사항이 없는 리로드(no-op reload)는 제공자 프롬프트 캐시 접두사를 바이트 단위로 동일하게 유지합니다.

## 성능 및 프롬프트 캐시

코드 런타임이 설치되어 있지 않으면 Agent는 기존 nil-디스패처 경로를 사용합니다. 사이드카 프로세스, JSON 인코딩, RPC, 이벤트 큐가 전혀 관여하지 않습니다. 런타임이 설치되면 Patty Code는 공유된 30초 세대 시작 예산 안에서 한 번에 최대 4개의 사이드카를 초기화합니다. 따라서 멈춘 선택적(optional) 런타임이 설치된 패키지 수만큼 부팅 또는 리로드 시간을 늘릴 수는 없습니다. 예산 안에서 시작하지 못한 패키지는 `runtime.required` 설정에 따라 성능이 저하되거나 실패합니다.
활성화된 동기 인터셉터는 의도적으로 해당 핫 경로(hot path)에 위치하며 직렬로 실행되므로 RPC와 핸들러 지연 시간이 합산됩니다. 입력, 도구, 권한, 제공자 인터셉터는 작고 결정적으로 유지하세요. 관찰(observation) 이벤트는 제한된 비차단 큐를 사용하며, 역압(backpressure) 상황에서는 턴을 지연시키는 대신 경고와 함께 삭제됩니다.

관찰 전용 확장은 제공자에게 보이는 캐시 접두사를 변경하지 않습니다. 안정적인 시스템 프롬프트 또는 도구 교체는 설치/리로드 후 의도된 콜드 접두사 하나를 생성하며 이후에는 계속 캐시 가능한 상태를 유지합니다. 시스템 프롬프트, 도구 스키마, 컨텍스트 접두사 또는 제공자 요청에 타임스탬프, 임의 값, 세션 ID 등 턴별 데이터를 주입하는 전략은 캐시 재사용을 파괴할 수 있습니다. 동적 데이터는 가능하면 현재 턴의 끝부분에 유지해야 합니다. 유지보수자는 다음 명령으로 호스트 오버헤드를 측정할 수 있습니다:

```bash
go test ./internal/extension/... -run '^$' -bench 'Extension|Dispatch' -benchmem
```

## 확장 개발하기

완전한 [`starterextension`](../sdk/go/examples/starterextension/README.md) 패키지부터 시작하세요. 이 패키지는 매니페스트, Sidecar 소스, 크로스 플랫폼 빌드 명령, 연결(linked) 설치, 첫 관찰 가능한 인터셉트를 한 디렉터리에 담고 있습니다. 일반적인 개발 루프는 다음과 같습니다:

1. `patty-plugin.json`에 `apiVersion: "patty-code.io/plugin/v2"`를 추가하고 `contributes`와 (선택 사항으로) `runtime`을 선언하세요 — [Plugin Packages](./PLUGIN_PACKAGES.md#manifest-v2-extensions)를 참조하세요.
2. Sidecar를 구현하세요. [Go SDK](../sdk/go/README.md)(표준 라이브러리만 사용)는 전송(transport), 핸드셰이크, 순서화(sequencing), 콘텐츠 참조, 종료를 처리합니다. [wire 계약](./EXTENSION_PROTOCOL.md)과 [생성된 메서드 인덱스](./EXTENSION_PROTOCOL.generated.md)는 언어 중립적인 참조 자료입니다.
3. 런타임 바이너리를 빌드하고, `patcode plugin install /path/to/plugin --dry-run`으로 신뢰도와 기능을 미리 확인한 다음 `--link --yes`로 설치하세요.
4. `patcode plugin doctor <name>`으로 검증하고, 유휴 상태에서 `/reload`를 실행한 다음 기여한 인터셉트, Provider, UI 액션 또는 리소스를 테스트하세요.

SDK 릴리스는 변경 불가능한 `sdk/go/vX.Y.Z` 태그를 사용합니다. 첫 공개 버전은 `sdk/go/v1.0.0`이며, 해당 태그가 생기기 전까지는 버전이 없는 모듈에 의존하지 말고 소스 체크아웃에서 스타터를 사용하세요.

## 호환성

- 네이티브 `patty-plugin.json` 매니페스트는 정확한 `patty-code.io/plugin/v2` API 버전을 선언해야 합니다. 확장 매니페스트가 v1에서 공개 릴리스된 적이 없으므로 v1 이중 읽기(dual-read)나 자동 마이그레이션 경로는 없습니다.
- 이전 버전의 Patty Code는 확장 전용 상태를 무시합니다: 세션별 `<session>.extensions.json` 사이드카 파일, `plugin/...` 모델 ref(단순히 사용할 수 없는 모델로 처리됨), `extension_surface` / `extension_status` 이벤트 종류(이전 프론트엔드는 알 수 없는 종류를 버립니다. `patty-code.extensionSurface`가 없는 ACP 클라이언트는 텍스트 폴백을 받습니다).
- `plugin-packages.json`은 기존 스키마를 유지합니다. 활성화된 설치 런타임이 곧 신뢰 기록(trust record)입니다.

## 보안 모델

코드 확장은 Patty Code 샌드박스 밖에서 필터링되지 않은 상속 환경으로 실행됩니다. 전체 세션과 환경을 읽을 수 있고, 권한 및 워크스페이스 제한을 우회하며, 머신을 직접 조작할 수 있습니다. `permission.decision`의 "allow"는 호스트의 거부(deny)를 무시합니다. 그 대가로 호스트는 다음을 강제합니다:

- 플러그인 흐름을 통해 설치된 플러그인만 런타임을 시작할 수 있습니다 — 프로젝트 구성으로는 절대 선언할 수 없습니다;
- 핸드셰이크는 매니페스트에 없는 모든 기능을 거부합니다;
- 교체는 각 지점의 DTO와 스키마에 대해 다시 검증됩니다;
- 사이드카 진단 정보, 구조화된 UI, 인터셉터 사유, 제공자 오류는 UI, 로그, 오류 화면에 도달하기 전에 호스트가 자격 증명을 삭제(redact)합니다. 일반 제공자/모델 콘텐츠는 제품 데이터로 보존됩니다;
- 충돌한 사이드카는 자체 작업을 명시적으로 실패 처리합니다 — Patty Code는 다른 모델이나 전략으로 조용히 폴백하지 않습니다.
