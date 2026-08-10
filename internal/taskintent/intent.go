package taskintent

import (
	"strings"
	"unicode/utf8"
)

type Intent uint8

const (
	Conversation Intent = iota
	Advisory
	ObservableRead
	Mutation
	PersistentAction
)

func Classify(input string) Intent {
	switch {
	case deliveryTaskHasMutationIntent(input):
		return Mutation
	case NeedsPersistentAction(input):
		return PersistentAction
	case deliveryTaskIsConversationOnly(input):
		return Conversation
	case !heuristicInputIsTask(input):
		return Conversation
	case deliveryTaskIsAdvisory(input):
		return Advisory
	default:
		return ObservableRead
	}
}

func (i Intent) NeedsEvidence() bool {
	return i == ObservableRead || i == Mutation || i == PersistentAction
}

func NeedsEvidence(input string) bool {
	return Classify(input).NeedsEvidence()
}

var deliveryMutationNeedles = []string{
	"fix", "repair", "resolve", "create", "add", "write", "edit", "update", "change", "delete", "remove", "rename",
	"implement", "refactor", "apply", "install", "publish", "commit", "push", "continue work",
	"modify", "patch", "replace", "move", "configure", "upgrade", "downgrade", "bump", "enable", "disable", "merge",
	"make changes", "make a change", "make the changes", "make the requested changes", "make the necessary changes", "make these changes", "make those changes", "make code changes",
	"복구", "해결", "생성하기", "새로 만들기", "추가", "작성", "편집", "수정", "업데이트", "삭제", "제거", "이름 재정의", "구현", "리팩토링",
	"시행", "실현", "설치", "배포", "커밋", "계속하기", "조정", "바꾸기", "이동", "업그레이드", "다운그레이드", "활성화", "비활성화", "병합", "변경", "패치",
}

var deliveryAdvisoryPhrases = []string{
	"what's wrong", "what is wrong", "why", "what should i do", "what can i do", "how should i", "how do i", "how can i",
	"can you explain", "could you explain", "give me advice", "any advice", "help me understand",
	"왜", "어떤 일", "어떻게 할까", "어떻게", "어떻게", "어떻게", "어떤 문제", "어떤 이유", "이유", "제안 해주세요", "어떤 제안",
}

func NeedsMutation(input string) bool {
	intent := Classify(input)
	return intent == Mutation || intent == PersistentAction
}

func deliveryTaskHasMutationIntent(input string) bool {
	affirmative, _ := deliveryTaskMutationIntent(input)
	return affirmative
}

func NeedsPersistentAction(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" {
		return false
	}
	actionNeedles := []string{
		"remember", "save", "store", "keep this", "keep that",
		"기억해요", "기록하세요", "저장", "저장하세요", "기록해두세요",
	}
	durableNeedles := []string{
		"permanently", "durable", "long-term", "long term", "across sessions", "future sessions", "every session", "after restart", "after restarting",
		"영구히", "장기간", "지속적으로", "세션 간", "앞으로 매번", "향후 세션", "재시작 후", "다음 시작 시",
	}
	for _, clause := range deliveryTaskClauses(normalized) {
		action := false
		for _, needle := range actionNeedles {
			affirmative, _ := deliveryTaskNeedleIntent(clause, needle)
			action = action || affirmative
		}
		durable := false
		for _, needle := range durableNeedles {
			affirmative, _ := deliveryTaskNeedleIntent(clause, needle)
			durable = durable || affirmative
		}
		if action && durable && !deliveryTaskClauseIsAdvisory(clause) {
			return true
		}
	}
	return false
}

func deliveryTaskIsConversationOnly(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))
	if normalized == "" || deliveryTaskHasHostAnchor(normalized) || deliveryTaskHasCommand(normalized) {
		return false
	}
	localCue := containsAnySubstring(normalized, []string{
		"next turn", "next message", "later in this chat", "this conversation", "when i ask again", "when i ask next",
		"다음 라운드", "다음 라운드", "다음 메시지", "나중에 물어보세요", "곧 다시 물어보세요", "이 대화", "이번 대화", "이번 라운드 세션",
	})
	conversationAction := containsAnySubstring(normalized, []string{
		"remember", "keep in mind", "keep this", "keep that", "answer", "respond", "reply",
		"기억해요", "기록하세요", "답변", "응답", "다시 알려주세요",
	})
	return localCue && conversationAction
}

func deliveryTaskMutationIntent(input string) (affirmative, negated bool) {
	normalized := strings.ToLower(strings.TrimSpace(input))
	for _, clause := range deliveryTaskClauses(normalized) {
		clauseAffirmative := false
		clauseNegated := false
		if deliveryMutationClauseNegated(clause) {
			clauseNegated = true
		}
		for _, needle := range deliveryMutationNeedles {
			hasAffirmative, hasNegated := deliveryTaskNeedleIntent(clause, needle)
			clauseAffirmative = clauseAffirmative || hasAffirmative
			clauseNegated = clauseNegated || hasNegated
		}
		if clauseAffirmative && deliveryTaskClauseIsAdvisory(clause) && !deliveryTaskAdvisoryClauseRequestsMutation(clause) {
			clauseAffirmative = false
			clauseNegated = true
		}
		affirmative = affirmative || clauseAffirmative
		negated = negated || clauseNegated
	}
	return affirmative, negated
}

func deliveryTaskIsAdvisory(input string) bool {
	normalized := strings.ToLower(strings.TrimSpace(input))

	if deliveryTaskHasHostAnchor(normalized) || deliveryTaskHasCommand(normalized) {
		return false
	}

	sawAdvisory := false
	for _, clause := range deliveryTaskClauses(normalized) {
		if deliveryTaskClauseIsAdvisory(clause) {
			sawAdvisory = true
			continue
		}
		if deliveryTaskClauseHasObservableWork(clause) {
			return false
		}
	}
	if sawAdvisory {
		return true
	}

	_, negatedMutation := deliveryTaskMutationIntent(normalized)
	return negatedMutation
}

func deliveryTaskHasHostAnchor(input string) bool {
	for _, anchor := range []string{
		"this repo", "this repository", "current repository", "codebase", "workspace", "pull request", "this pr", "ci job",
		"/pull/", "actions/runs/",
		"현재 저장소", "이 저장소", "현재 프로젝트", "이 프로젝트", "코드베이스", "작업 공간", "이 pr", "이 pr", "이 pr", "이 pr",
	} {
		if strings.Contains(input, anchor) {
			return true
		}
	}
	return deliveryTaskHasFileReference(input)
}

func deliveryTaskHasFileReference(input string) bool {
	previous := rune(0)
	for index, current := range input {
		if current == '@' && index+1 < len(input) &&
			(index == 0 || strings.ContainsRune(" \t\r\n([{<,:;（【《，。；：", previous)) {
			next, _ := utf8.DecodeRuneInString(input[index+1:])
			if !strings.ContainsRune(" \t\r\n", next) {
				return true
			}
		}
		previous = current
	}

	for _, raw := range strings.FieldsFunc(input, func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', '`', '\'', '"', '(', ')', '[', ']', '{', '}', '<', '>', ',', '，', ';', '；', '!', '！', '?', '？':
			return true
		default:
			return false
		}
	}) {
		token := strings.ToLower(strings.TrimSpace(raw))
		if token == "" || strings.Contains(token, "://") {
			continue
		}
		if strings.HasPrefix(token, "./") || strings.HasPrefix(token, "../") ||
			strings.HasPrefix(token, "/") || strings.Contains(token, `\`) {
			return true
		}
		base := token
		if slash := strings.LastIndexByte(base, '/'); slash >= 0 {
			base = base[slash+1:]
		}
		switch base {
		case "dockerfile", "makefile", "cmakelists.txt", "justfile", "license", "readme", "changelog":
			return true
		}
		dot := strings.LastIndexByte(base, '.')
		if dot < 0 {
			continue
		}
		switch base[dot:] {
		case ".go", ".mod", ".sum", ".js", ".jsx", ".ts", ".tsx", ".py", ".rs", ".java", ".kt", ".swift",
			".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".rb", ".php", ".sh", ".zsh", ".fish", ".ps1",
			".md", ".json", ".yaml", ".yml", ".toml", ".xml", ".sql", ".proto", ".html", ".css", ".scss",
			".vue", ".svelte", ".txt", ".log", ".csv", ".pdf", ".env", ".ini", ".conf", ".lock":
			return true
		}
	}
	return false
}

func deliveryTaskHasCommand(input string) bool {
	tokens := strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		asciiWord := r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
		return !asciiWord && r != '_' && r != '-' && r != '.' && r != '/' && r != '\\' && r != ':'
	})
	for i := range tokens {
		if deliveryCommandStartsAt(tokens, i) {
			return true
		}
	}
	return false
}

func deliveryCommandStartsAt(tokens []string, index int) bool {
	command := strings.TrimSpace(tokens[index])
	if command == "" {
		return false
	}
	if strings.HasPrefix(command, "./") || strings.HasPrefix(command, "../") ||
		strings.HasPrefix(command, "/") || strings.Contains(command, `\`) {
		return true
	}
	next := ""
	if index+1 < len(tokens) {
		next = tokens[index+1]
	}
	if next != "--" && len(next) > 1 && strings.HasPrefix(next, "-") {
		return true
	}
	previous := ""
	if index > 0 {
		previous = tokens[index-1]
	}
	switch command {
	case "go":
		switch next {
		case "build", "clean", "doc", "env", "fmt", "generate", "get", "install", "list", "mod", "run", "test", "tool", "version", "vet", "work":
			return true
		}
	case "git", "npm", "npx", "pnpm", "yarn", "bun", "deno", "cargo", "rustc", "python", "python3",
		"bash", "sh", "zsh", "fish", "powershell", "pwsh", "docker", "docker-compose", "kubectl", "helm", "terraform",
		"gradle", "gradlew", "mvn", "dotnet", "xcodebuild", "gcc", "g++", "clang", "clang++":
		return deliveryCommandHasExplicitCue(previous) || deliveryCommandHasSubcommand(next)
	case "node":
		return deliveryCommandHasExplicitCue(previous) || next == "inspect" || next == "test"
	case "swift":
		return next == "build" || next == "package" || next == "run" || next == "test"
	case "make", "just":
		switch next {
		case "all", "build", "check", "clean", "fail", "failed", "failing", "install", "lint", "test":
			return true
		}
	case "pytest", "cmake", "ninja", "eslint", "tsc", "vitest", "jest":
		return deliveryCommandHasExplicitCue(previous) || next == "fail" || next == "failed" || next == "failing"
	}
	return false
}

func deliveryCommandHasExplicitCue(previous string) bool {
	switch previous {
	case "command", "execute", "executing", "run", "running", "using", "with":
		return true
	default:
		return false
	}
}

func deliveryCommandHasSubcommand(next string) bool {
	switch next {
	case "add", "apply", "branch", "build", "check", "checkout", "clean", "clone", "commit", "config", "container",
		"deploy", "describe", "destroy", "dev", "diff", "down", "env", "exec", "fetch", "fmt", "generate", "get", "image",
		"init", "install", "lint", "list", "log", "logs", "login", "logout", "merge", "mod", "package", "plan", "ps", "publish",
		"pull", "push", "rebase", "remote", "remove", "reset", "restore", "run", "serve", "show", "start", "stash", "status",
		"switch", "tag", "test", "tool", "uninstall", "up", "update", "upgrade", "version", "vet", "work", "worktree":
		return true
	default:
		return false
	}
}

func deliveryTaskClauseHasObservableWork(clause string) bool {
	for _, needle := range []string{
		"review", "inspect", "analyze", "check", "reproduce", "audit", "verify",
		"검토", "검사", "분석", "재현", "감사", "검증",
	} {
		affirmative, _ := deliveryTaskNeedleIntent(clause, needle)
		if affirmative {
			return true
		}
	}
	return false
}

func deliveryTaskClauseIsAdvisory(clause string) bool {
	for _, phrase := range deliveryAdvisoryPhrases {
		if strings.Contains(clause, phrase) {
			return true
		}
	}
	return false
}

func deliveryTaskAdvisoryClauseRequestsMutation(clause string) bool {
	advisoryIndex := len(clause)
	for _, phrase := range deliveryAdvisoryPhrases {
		if index := strings.Index(clause, phrase); index >= 0 && index < advisoryIndex {
			advisoryIndex = index
		}
	}
	if advisoryIndex == len(clause) {
		return false
	}
	if deliveryTaskStartsWithMutation(clause[:advisoryIndex]) {
		return true
	}

	for _, cue := range []string{" please ", " then ", " so ", " therefore ", "그런 다음", "따라서", "그런데", "전환하여"} {
		for rest := clause[advisoryIndex:]; ; {
			index := strings.Index(rest, cue)
			if index < 0 {
				break
			}
			rest = rest[index+len(cue):]
			if deliveryTaskStartsWithMutation(rest) {
				return true
			}
		}
	}
	for rest, offset := clause[advisoryIndex:], advisoryIndex; ; {
		index := strings.Index(rest, "부탁드립니다")
		if index < 0 {
			break
		}
		absolute := offset + index
		after := clause[absolute+len("부탁드립니다"):]
		requestWord := strings.HasSuffix(clause[:absolute], "신청") || strings.HasPrefix(after, "요청")
		if !requestWord && deliveryTaskStartsWithMutation(after) {
			return true
		}
		offset = absolute + len("부탁드립니다")
		rest = clause[offset:]
	}

	for _, cue := range []string{" and ", "그리고", "그리고"} {
		if index := strings.LastIndex(clause[advisoryIndex:], cue); index >= 0 {
			cueStart := advisoryIndex + index
			tail := clause[cueStart+len(cue):]
			if !deliveryTaskClauseHasNegation(clause[:cueStart]) && deliveryTaskStartsWithMutation(tail) {
				return true
			}
		}
	}
	return false
}

func deliveryTaskStartsWithMutation(input string) bool {
	input = strings.TrimSpace(input)
	for {
		stripped := false
		for _, prefix := range []string{"please ", "can you ", "could you ", "would you ", "you should ", "도와주세요", "부탁드립니다", "바로", "계속", "다시"} {
			if after, ok := strings.CutPrefix(input, prefix); ok {
				input = strings.TrimSpace(after)
				stripped = true
				break
			}
		}
		if !stripped {
			break
		}
	}
	for _, needle := range deliveryMutationNeedles {
		if containsTaskNeedle(input, needle) {
			if containsNonASCII(needle) {
				return strings.HasPrefix(input, needle)
			}
			tokens := strings.FieldsFunc(input, func(r rune) bool {
				return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_' && r != '\''
			})
			needleTokens := strings.Fields(needle)
			if len(tokens) >= len(needleTokens) {
				matches := true
				for i := range needleTokens {
					matches = matches && tokens[i] == needleTokens[i]
				}
				if matches {
					return true
				}
			}
		}
	}
	return false
}

func deliveryTaskClauseHasNegation(clause string) bool {
	clause = strings.ReplaceAll(clause, "’", "'")
	for _, phrase := range []string{
		" not ", " never ", " without ", "cannot", "can't", " cant ", "don't", " dont ", "won't", " wont ", "unable",
		"하지 마세요", "하지 마세요", "하지 마세요", "할 수 없습니다", "불가능합니다", "하고 싶지 않음", "두려움", "불필요", "불필요", "불가능", "방법 없음", "없음", "금지", "거절",
	} {
		if strings.Contains(" "+clause+" ", phrase) {
			return true
		}
	}
	return false
}

func deliveryTaskClauses(input string) []string {
	input = strings.NewReplacer(
		" but ", "\n",
		" however ", "\n",
		" nevertheless ", "\n",
		"하지만 부탁드립니다", "\n부탁드립니다",
		"하지만", "\n",
		"그런데", "\n",
	).Replace(input)
	return strings.FieldsFunc(input, func(r rune) bool {
		switch r {
		case '\n', '\r', '.', '。', ',', '，', ';', '；', '!', '！', '?', '？':
			return true
		default:
			return false
		}
	})
}

func deliveryMutationClauseNegated(clause string) bool {
	for _, phrase := range []string{
		"without changing", "without modifying", "analysis only", "review only",
		"변경하지 마세요", "분석만 하세요", "분석만 하세요", "검사만 하세요", "검사만 하세요", "검토만 하세요", "검토만 하세요",
	} {
		if strings.Contains(clause, phrase) {
			return true
		}
	}
	return false
}

func deliveryTaskNeedleIntent(clause, needle string) (affirmative, negated bool) {
	if containsNonASCII(needle) {
		for offset := 0; offset < len(clause); {
			relative := strings.Index(clause[offset:], needle)
			if relative < 0 {
				break
			}
			index := offset + relative
			prefix := []rune(clause[:index])
			suffix := []rune(clause[index+len(needle):])
			if deliveryMutationRunesNegated(prefix) || deliveryMutationRunesSuffixNegated(suffix) {
				negated = true
			} else {
				affirmative = true
			}
			offset = index + len(needle)
		}
		return affirmative, negated
	}

	clause = strings.ReplaceAll(clause, "’", "'")
	tokens := strings.FieldsFunc(clause, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '_' && r != '\''
	})
	needleTokens := strings.Fields(needle)
	for i := 0; i+len(needleTokens) <= len(tokens); i++ {
		matches := true
		for j, token := range needleTokens {
			if tokens[i+j] != token {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		if deliveryMutationTokensNegated(tokens[:i]) {
			negated = true
		} else {
			affirmative = true
		}
	}
	return affirmative, negated
}

func deliveryMutationTokensNegated(prefix []string) bool {
	if len(prefix) > 6 {
		prefix = prefix[len(prefix)-6:]
	}
	boundary := -1
	for i, token := range prefix {
		switch token {
		case "but", "however", "nevertheless", "instead", "so", "then", "therefore", "please":
			boundary = i
		}
	}
	if boundary >= 0 {
		prefix = prefix[boundary+1:]
	}
	for i, token := range prefix {
		if token == "not" && i+1 < len(prefix) && prefix[i+1] == "only" {
			continue
		}
		switch token {
		case "not", "never", "without", "cannot", "can't", "cant", "don't", "dont", "won't", "wont", "unable", "avoid", "avoiding", "afraid", "refuse", "refusing", "needn't":
			return true
		case "no":
			if i+1 < len(prefix) && prefix[i+1] == "need" {
				return true
			}
		}
	}
	return false
}

func deliveryMutationRunesNegated(prefix []rune) bool {
	if len(prefix) > 12 {
		prefix = prefix[len(prefix)-12:]
	}
	window := string(prefix)
	scopeStart := 0
	for _, boundary := range []string{"따라서", "그런 다음", "그런데", "전환하여", "변경하여"} {
		if index := strings.LastIndex(window, boundary); index >= 0 {
			end := index + len(boundary)
			if end > scopeStart {
				scopeStart = end
			}
		}
	}
	if index := strings.LastIndex(window, "부탁드립니다"); index >= 0 {
		before, after := window[:index], window[index+len("부탁드립니다"):]
		requestWord := strings.HasSuffix(before, "신청") || strings.HasPrefix(after, "요청")
		negatedRequest := false
		for _, marker := range []string{"하지 마세요", "할 수 없습니다", "불가능합니다", "하고 싶지 않음", "두려움", "불필요", "불필요", "불가능", "방법 없음", "금지", "거절"} {
			if strings.HasSuffix(before, marker) || strings.Contains(after, marker) {
				negatedRequest = true
				break
			}
		}
		if !requestWord && !negatedRequest && index+len("부탁드립니다") > scopeStart {
			scopeStart = index + len("부탁드립니다")
		}
	}
	window = window[scopeStart:]
	for _, marker := range []string{"하지 마세요", "할 수 없습니다", "불가능합니다", "하고 싶지 않음", "두려움", "불필요", "불가능", "방법 없음", "없음", "금지", "거절"} {
		if strings.Contains(window, marker) {
			return true
		}
	}
	return false
}

// deliveryMutationRunesSuffixNegated reports Korean verb-suffix negation that
// follows the needle ("복구하지 마세요" negates "복구"). Korean negates with
// suffixes, so the runes after the needle decide intent when the prefix does
// not.
func deliveryMutationRunesSuffixNegated(suffix []rune) bool {
	if len(suffix) > 12 {
		suffix = suffix[:12]
	}
	window := string(suffix)
	// Trim at the first clause boundary so a later clause can't bleed in.
	for _, boundary := range []string{"그리고", "또한", "다만", "그러나"} {
		if index := strings.Index(window, boundary); index >= 0 {
			window = window[:index]
			break
		}
	}
	for _, marker := range []string{"하지 마세요", "하지 말아 주세요", "하지 말아주세요", "하지 마라", "하지 말 것", "하지 않고", "하지 않기", "하지 않도록", "말고", "말아 주세요", "말아주세요", "안 하세요", "안 합니다", "않습니다", "않아요"} {
		if strings.Contains(window, marker) {
			return true
		}
	}
	return false
}

func containsAnySubstring(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}
