# Cache-Aware Context Projection과 지연 압축

> 날짜: 2026-08-07
> 상태: 현재 구현 설명
> 핵심 제약: canonical transcript는 영구적인 사실 원본이며, 캐시 상태는 비용 전략에만 영향을 주고 기록 자체를 직접 다시 쓰지 않는다.

## 1. 문제와 목표

긴 세션은 동시에 두 가지 목표를 만족해야 한다.

1. 복구, 되감기, 브랜치, 감사에 필요한 전체 기록을 보존한다.
2. 모델 컨텍스트가 한계에 가까워질 때 더 짧고 안정적인 provider-visible 요청을 만든다.

기존 경로는 cache TTL 만료와 session 압축을 묶어 두었다. cold resume가 발생하면 동기 압축과 함께 기록을 다시 썼다. 이 방식은 비용 신호를 데이터 변경 신호로 바꾸고, 사용자가 단순히 세션을 다시 여는 상황에서도 기존 prefix를 깨뜨렸다.

현재 설계는 이 둘을 분리한다.

```text
canonical transcript (Session.Messages, 일반 압축은 이 기록을 다시 쓰지 않음)
    |
    +-- model-visible context projection
    |
    +-- cache state (warm/cold/unknown, 비용과 관측에만 사용)
```

## 2. 영속 저장 경계

### Canonical transcript

- `Session.Messages`는 항상 전체 transcript를 저장한다.
- 일반 compaction, cold resume, tool prune/snip은 canonical 메시지를 삭제하거나 교체하지 않는다.
- rewind, fork, branch는 계속 canonical transcript를 사실 원본으로 사용한다.

### Context projection sidecar

- projection은 `<session>.context.json`에 저장되며, 원래 session 파일 형식은 바꾸지 않는다.
- sidecar는 projection, covered prefix fingerprint, transcript/projection version, prompt cache key, cache 상태, compaction telemetry를 저장한다.
- session을 삭제할 때 sidecar도 같은 삭제 대상에 포함된다.
- 이전 버전이 sidecar를 모를 경우에도 전체 session은 그대로 읽을 수 있어야 한다. 새 버전은 sidecar가 없거나 schema가 오래되었거나 검증에 실패하면 안전하게 다시 만든다.

## 3. 런타임 동작

### Resume는 캐시 상태만 기록한다

세션을 다시 열 때 provider TTL과 마지막 활동 시각을 기준으로 `warm`, `cold`, `unknown` 상태만 기록한다. Resume 경로는 `Compact`, `SnapshotRewrite`, `PruneStaleToolResults`를 호출하지 않으며 canonical transcript도 수정하지 않는다.

### Preflight에서 projection을 지연 생성한다

각 모델 요청 직전에 `contextPreflight`가 현재 token 압력을 보고 projection 필요 여부를 판단한다.

- 압력 임계치에 도달하지 않으면 append-only canonical view를 그대로 보낸다.
- 압축 임계치에 도달하면 projection 생성과 설치를 시도한다.
- force 임계치에 도달했지만 접을 수 있는 내용이 없으면, tool loop가 아닐 때 재시도 가능한 `ErrCompactionRequired`를 반환한다.
- 요약 생성에 실패하면 mechanical marker를 쓰지 않고, 반제품 projection도 설치하지 않으며, canonical도 수정하지 않는다.
- tool loop 중에는 notice만 보내고 이후 preflight나 stuck guard가 처리하게 두어 tool call 짝이 끊기지 않게 한다.

### Provider-visible 순서

projection은 다음의 안정적인 순서를 유지한다.

```text
system
-> 결정적인 초기 user turns
-> 단일 rolling summary
-> 반드시 유지해야 하는 메시지
-> recent tail
```

초기 user turn의 자격은 고정 token/char 추정과 context-window 상한으로 계산되며, 최근 provider usage에 의존하지 않는다. 동적 usage 보정은 tail 크기 같은 메시지 정체성을 바꾸지 않는 추정에만 사용한다. 따라서 projection이 활성화된 뒤에도 초기 prefix는 canonical/projection 통계 차이 때문에 흔들리지 않는다.

이전 summary는 다음 fold에 흡수되고, 새 summary가 rolling 방식으로 이어받는다. provider-visible projection에는 항상 summary가 하나만 유지되며 무한한 요약 체인이 생기지 않는다. 원래 summary는 canonical transcript 안에 그대로 남는다.

## 4. 유효성 및 무효화

projection은 fail-closed 검증을 사용한다.

- `CoveredPrefixHash`는 `ModelMessages(canonical[:CoveredCount])`의 전체 provider-visible 내용을 기준으로 안정적인 fingerprint를 만든다. 여기에는 이미지, Patty Code 메타데이터, Responses item, tool call ID/name/arguments/thought signature가 포함된다.
- `PromptCacheKey`는 반드시 존재해야 하며 `workspace|session lineage|model`과 정확히 일치해야 한다.
- fingerprint가 없거나, prefix가 edit/rewrite되었거나, 모델 또는 lineage가 바뀌면 메모리 projection은 즉시 무효화된다.
- rewind, fork, branch, snip/prune, 명시적 범위 요약은 관련 projection을 무효화하거나 격리한다.

로딩 시 어떤 모델의 sidecar key가 일치하지 않으면 현재 메모리 상태만 버리고 디스크 파일은 지우지 않는다. 다른 모델이 여전히 그 파일을 사용할 수 있기 때문이다.

## 5. Provider compaction과 실패 전략

Provider 인터페이스는 이미 다음을 정의하고 있다.

- `NativeCompactor`
- `CompactionRequest` / `CompactionResult`
- `CompactionCapabilities`
- `ErrCompactionUnsupported`

현재 Responses vendor는 명시적으로 unsupported를 반환하고 Patty Code의 요약 경로로 되돌아간다. 요약이 처음 실패한 뒤의 재시도는 두 번의 attempt에 걸친 usage와 request count를 합산해 비용과 telemetry에 반영한다.

Anthropic, DeepSeek 등의 native compaction endpoint는 아직 연결되지 않았다. capability 인터페이스가 있다고 해서 그 endpoint가 이미 사용 가능하다는 뜻은 아니다.

## 6. 캐시 영향

- 긍정적 효과: cold resume가 더 이상 TTL 상태 때문에 기록을 다시 쓰지 않으므로, 캐시가 여전히 warm일 때 기존 append-only prefix를 계속 재사용할 수 있다.
- 예상 miss: 높은 압력에서 projection이 처음 활성화될 때는 요청 prefix가 canonical에서 `summary + tail`로 바뀌므로 한 번의 예상 가능한 cache miss가 발생한다.
- 안정성: 활성화 후에는 결정적인 초기 turn, 단일 rolling summary, 안정적인 cache key, fail-closed fingerprint가 의미 없는 prefix drift를 줄인다.
- 불확실성: token 추정 때문에 preflight가 이전 경로보다 더 빠르거나 더 늦게 요약을 시작할 수 있으므로, break-even 비용은 telemetry로 계속 관찰해야 한다.

## 7. 아직 구현되지 않은 후속 항목

다음 기능은 현재 단계에 포함되지 않으며, 이미 동작한다고 가정하면 안 된다.

1. Anthropic/DeepSeek native compaction endpoint
2. compaction 이후 `SaveKnowledge` 호출 또는 EventChain 기록
3. EventChain 기반의 cross-session L2 자동 복구
4. feature flag 관찰 기간과 이전 호환 경로의 최종 정리
5. 완전한 break-even 비용 대시보드 집계

이 후속 작업들은 각각 실패 원자성, 영속 저장 호환성, 캐시 영향, provider capability 탐지를 별도로 설계해야 한다. 다시 cache TTL과 canonical transcript 재작성 로직을 묶어서는 안 된다.
