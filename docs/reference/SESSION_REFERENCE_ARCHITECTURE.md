# 세션 참조 기능 아키텍처 문서

> Historical issue reference: this design is retained for context; current
> issues live at https://github.com/patrickrho-patty/patty-code/issues.

## 1. 기능 요구사항

현재 세션에서 `@`를 사용해 다른 세션의 대화 기록을 참조하여 AI에 콘텐츠로 전송합니다.

### 1.1 사용자 시나리오

- 사용자가 현재 세션에서 이전 세션의 대화 내용을 참조하려는 경우
- 사용자가 AI가 이전 대화의 컨텍스트를 참고하길 원하는 경우
- 사용자가 여러 세션의 논의 결과를 통합하려는 경우

### 1.2 기대 동작(P0-MVP)

```
입력 @
→ 기존 메뉴 표시(파일/디렉터리 목록)
→ 메뉴 상단에 "past:chats" 옵션 추가
→ past:chats 선택
→ 과거 세션 목록으로 전환
→ 세션 하나 선택
→ Composer 상단에 "참조된 세션" 표시
→ 전송 시 해당 세션의 과거 내용을 현재 prompt/context에 첨부
```

---

## 2. 기존 코드 구조 분석

### 2.1 핵심 파일

| 파일 | 용도 |
|------|------|
| `desktop/frontend/src/components/Composer.tsx` | 입력 상자 컴포넌트, @ 기능 포함 |
| `desktop/frontend/src/components/FileMenu.tsx` | @ 파일 메뉴 컴포넌트 |
| `desktop/frontend/src/components/HistoryPanel.tsx` | 과거 세션 패널 |
| `desktop/frontend/src/lib/bridge.ts` | 프론트엔드-백엔드 통신 인터페이스 |
| `desktop/frontend/src/lib/types.ts` | 타입 정의 |

### 2.2 기존 @ 기능 구현

**Composer.tsx 257-337행**에 파일 참조 기능이 구현되어 있습니다:

```typescript
// --- @ file references ---
const atRaw = useMemo(() => {
  const m = /(?:^|\s)@([^\s]*)$/.exec(text);
  return m ? m[1] : null;
}, [text]);

// [파일 매칭 결과]
const atMatches = useMemo(() => {
// [로컬 디렉터리 및 검색 결과 필터링]
}, [atRaw, atFrag, entries, searchEntries]);

// [메뉴 모드 판단]
const menuMode: "slash" | "slasharg" | "at" | null = ...;

// [파일 메뉴 렌더링]
{menuMode === "at" && <FileMenu items={atMatches} ... />}
```

### 2.3 기존 세션 API(재사용 가능)

```typescript
interface AppBindings {
// [세션 목록]
  ListSessions(): Promise<SessionMeta[]>;

  // 세션 작업(기록 읽기 재사용 가능)
  PreviewSession(path: string): Promise<HistoryMessage[]>;
}
```

---

## 3. P0-MVP 구현 방안

### 3.1 설계 방향

새로운 `@session:` 구문을 만드는(create) 대신, 기존 `@` 메뉴에 "past:chats" 옵션을 추가합니다.

**메뉴 구조:**
```
@
├── 📁 past:chats        ← 신규: 선택 시 과거 세션 목록 표시
├── 📁 src/
├── 📁 docs/
├── 📄 README.md
└── ...
```

### 3.2 구현 로드맵

```
1단계: 백엔드에 검색 API 추가
    ↓
2단계: 프론트엔드 bridge.ts에 인터페이스 노출
    ↓
3단계: @ 메뉴에 "past:chats" 옵션 추가
    ↓
4단계: past:chats 선택 후 세션 목록으로 전환
    ↓
5단계: 세션 선택 후 참조 영역에 추가
    ↓
6단계: 전송 시 세션 컨텍스트 첨부
```

### 3.3 최소 수정 파일 목록

```
desktop/frontend/src/lib/types.ts      — SessionReference 타입 추가
desktop/frontend/src/lib/bridge.ts     — SearchSessions API 추가
desktop/frontend/src/components/Composer.tsx — @ 메뉴 로직 확장
desktop/frontend/src/components/FileMenu.tsx — 세션 항목을 지원하도록 메뉴 확장
desktop/app.go                         — SearchSessions 메서드 추가
desktop/sessions.go                    — 세션 검색 로직 구현
```

### 3.4 타입 정의

```typescript
// types.ts
export interface SessionReference {
  path: string;
  title: string;
  preview?: string;
  turns?: number;
  createdAt?: number;
  lastActivityAt?: number;
  messages?: HistoryMessage[]; // P0에서는 저장하지 않고, 전송 시에 가져옴
}
```

### 3.5 API 설계

```typescript
// bridge.ts
interface AppBindings {
  // 신규: 세션 검색
  SearchSessions(query: string): Promise<SessionMeta[]>;

  // 기존: 세션 기록 읽기(재사용)
  PreviewSession(path: string): Promise<HistoryMessage[]>;
}
```

### 3.6 프론트엔드 로직 수정

**Composer.tsx 수정:**

```typescript
// 1. 상태 추가
const [showPastChats, setShowPastChats] = useState(false);
const [pastChats, setPastChats] = useState<SessionMeta[]>([]);
const [sessionRefs, setSessionRefs] = useState<SessionReference[]>([]);

// 2. @ 메뉴 렌더링 수정
{menuMode === "at" && (
  showPastChats ? (
// [세션 목록 표시]
    <SessionMenu
      items={pastChats}
      activeIndex={active}
      onPick={pickSession}
      onHover={setActive}
    />
  ) : (
    // 파일 목록 표시(기존 로직)
    <>
      <button
        className="slashmenu__item slashmenu__item--special"
        onMouseDown={() => {
          setShowPastChats(true);
          app.ListSessions().then(setPastChats);
        }}
      >
        <MessageSquare size={13} />
        <span className="slashmenu__name">past:chats</span>
        <span className="slashmenu__desc">과거 세션 참조</span>
      </button>
      <FileMenu items={atMatches} ... />
    </>
  )
)}

// 3. 세션 선택 후 처리
const pickSession = (session: SessionMeta) => {
// [참조 영역에 추가]
  setSessionRefs(prev => [...prev, {
    path: session.path,
    title: session.title || session.preview || "Untitled",
    preview: session.preview,
    turns: session.turns,
    createdAt: session.createdAt,
    lastActivityAt: session.lastActivityAt,
  }]);

// [상태 초기화]
  setShowPastChats(false);
  setText(""); // 입력 상자 비우기
};

// 4. 전송 시 세션 컨텍스트 첨부
const handleSubmit = async () => {
  let context = "";

  if (sessionRefs.length > 0) {
    context = "다음은 사용자가 참조한 과거 세션 컨텍스트입니다.\n\n";
    for (const ref of sessionRefs) {
      const messages = await app.PreviewSession(ref.path);
      const limited = limitMessages(messages, 30, 20000);
      context += formatSessionContext(ref.title, limited);
    }
    context += "\n\n현재 사용자 질문:\n";
  }

  onSubmit(context + text);
};
```

### 3.7 제한 정책

```
최근 메시지 최대 30개 참조
또는 최대 20k자
초과분은 잘라내고 "잘렸습니다" 표시
```

### 3.8 전송 시 메시지 형식

```
다음은 사용자가 참조한 과거 세션 컨텍스트입니다:

[세션: 로그인 버그 수정]
사용자: ...
어시스턴트: ...
사용자: ...

현재 사용자 질문:
...
```

---

## 4. 아키텍처 다이어그램

```
┌─────────────────────────────────────────────────────────────┐
│                      Composer.tsx                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  @ 메뉴 로직                                          │    │
│  │  - 기존: 파일/디렉터리 목록                             │    │
│  │  - 신규: past:chats 옵션(메뉴 상단)                    │    │
│  └─────────────────────────────────────────────────────┘    │
│                           │                                 │
│                           ▼                                 │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  메뉴 렌더링                                          │    │
│  │  - showPastChats=false → FileMenu + past:chats 버튼 │    │
│  │  - showPastChats=true → SessionMenu(세션 목록)        │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      bridge.ts                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  신규 API:                                            │    │
│  │  - SearchSessions(query): Promise<SessionMeta[]>     │    │
│  │                                                       │    │
│  │  재사용 API:                                          │    │
│  │  - ListSessions(): Promise<SessionMeta[]>            │    │
│  │  - PreviewSession(path): Promise<HistoryMessage[]>   │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    desktop/app.go                           │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  신규 메서드:                                         │    │
│  │  - SearchSessions(query string) []SessionMeta        │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

---

## 5. UI 설계

### 5.1 @ 메뉴(showPastChats=false)

```
┌─────────────────────────────────────────────────────────────┐
│  @  ← 사용자 입력                                            │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  💬 past:chats        과거 세션 참조                   │    │
│  ├─────────────────────────────────────────────────────┤    │
│  │  📁 src/                                               │    │
│  │  📁 docs/                                              │    │
│  │  📄 README.md                                          │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  [메시지 입력...]                                    [전송]  │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 세션 목록(showPastChats=true)

```
┌─────────────────────────────────────────────────────────────┐
│  @past:chats  ← 사용자 입력                                  │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  💬 프로젝트 아키텍처 설계 논의 - 2026-06-04          │    │
│  │  💬 데이터 처리 방안 - 2026-06-03                    │    │
│  │  💬 API 인터페이스 설계 - 2026-06-02                 │    │
│  │  ← 파일 목록 반환(return)                             │    │
│  └─────────────────────────────────────────────────────┘    │
│                                                             │
│  [메시지 입력...]                                    [전송]  │
└─────────────────────────────────────────────────────────────┘
```

### 5.3 참조 영역

```
┌─────────────────────────────────────────────────────────────┐
│  📎 참조된 세션:                                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │  📄 프로젝트 아키텍처 설계 논의 (8턴)      [×]         │    │
│  └─────────────────────────────────────────────────────┘    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  [메시지 입력...]                                    [전송]  │
└─────────────────────────────────────────────────────────────┘
```

---

## 6. P1 후속 개선(추후 구현)

- 단일/다중 메시지 선택
- hover로 세션 상세 미리보기
- 잘라내기/캐시 최적화
- 다국어(i18n) 번역
- 세션 검색 기능

---

## 7. 수용 기준

### P0 수용 기준

- [ ] `@` 입력 시 메뉴가 표시되고 "past:chats" 옵션 포함
- [ ] "past:chats" 선택 시 과거 세션 목록 표시
- [ ] 세션 선택 후 참조 영역에 표시
- [ ] 참조한 세션을 삭제할 수 있음
- [ ] 전송 시 세션 컨텍스트가 올바르게 첨부됨
- [ ] 참조 내용이 메시지 30개 또는 20k자 이내로 제한됨
- [ ] 초과분은 잘라내고 안내 표시

### 테스트 케이스

- [ ] 과거 세션이 없을 때의 동작
- [ ] 매우 큰 세션 참조 시 잘라내기
- [ ] 기존 @ 파일 참조와 동시 사용
- [ ] 다양한 테마에서의 표시 효과
