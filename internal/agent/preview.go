package agent

import (
	"encoding/json"
	"regexp"
	"strings"

	"patty/internal/provider"
)

//
var TransientUserBlockTags = []string{
	"response-language",
	"reasoning-language",
	"memory-update",
	"background-jobs",
	"active-goal",
	"autoresearch-runtime",
	"hook-context",
	"capability-route",
	"interrupted-turn-recovery",
}

var reTransientUserBlock = buildTransientUserBlockRE(TransientUserBlockTags)

func buildTransientUserBlockRE(tags []string) *regexp.Regexp {
	alt := strings.Join(tags, "|")
	return regexp.MustCompile(`(?s)^\s*<(?:` + alt + `)(?:\s+[^>]*)?>.*?</(?:` + alt + `)>\s*\n?`)
}

func stripTrailingDeliveryRuntime(s string) string {
	trimmed := strings.TrimRight(s, " \t\r\n")
	if cut, ok := strings.CutSuffix(trimmed, DeliveryRuntimeMarker); ok {
		return strings.TrimRight(cut, " \t\r\n")
	}
	return s
}

const memoryCompilerExecutionOpen = "<memory-compiler-execution>"

var reMemoryCompilerExecution = regexp.MustCompile(`(?s)<memory-compiler-execution>\s*(.*?)\s*</memory-compiler-execution>`)

func ContainsMemoryCompilerExecution(content string) bool {
	return strings.Contains(content, memoryCompilerExecutionOpen)
}

//
func StripTransientUserBlocks(content string) string {
	s := unwrapMemoryCompilerExecution(content)
	for {
		next := reTransientUserBlock.ReplaceAllStringFunc(s, func(string) string {
			return ""
		})
		if next == s {
			break
		}
		s = next
	}
	s = stripTrailingDeliveryRuntime(s)
	s = stripTrailingMemoryRecall(s)
	return strings.TrimLeft(s, " \t\r\n")
}

func stripTrailingMemoryRecall(s string) string {
	trimmed := strings.TrimRight(s, " \t\r\n")
	const open = "<memory-recall>"
	const close = "</memory-recall>"
	if !strings.HasSuffix(trimmed, close) {
		return s
	}
	if index := strings.LastIndex(trimmed, open); index >= 0 {
		return strings.TrimRight(trimmed[:index], " \t\r\n")
	}
	return s
}

func unwrapMemoryCompilerExecution(content string) string {
	const maxDepth = 24
	for range maxDepth {
		if !ContainsMemoryCompilerExecution(content) {
			return content
		}
		next := reMemoryCompilerExecution.ReplaceAllStringFunc(content, func(block string) string {
			m := reMemoryCompilerExecution.FindStringSubmatch(block)
			if len(m) < 2 {
				return ""
			}
			return memoryCompilerSourceEvent(m[1])
		})
		if next == content {
			break // no complete block matched (e.g. a dangling/truncated tag)
		}
		content = next
	}
	if idx := strings.Index(content, memoryCompilerExecutionOpen); idx >= 0 {
		content = strings.TrimRight(content[:idx], " \t\r\n")
	}
	return content
}

func memoryCompilerSourceEvent(body string) string {
	var contract struct {
		SourceEvent string `json:"source_event"`
		PlannerIR   struct {
			SourceEvent string `json:"source_event"`
		} `json:"planner_ir"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &contract); err != nil {
		return ""
	}
	if s := strings.TrimSpace(contract.PlannerIR.SourceEvent); s != "" {
		return s
	}
	return strings.TrimSpace(contract.SourceEvent)
}

func UserPreviewText(content string) string {
	s := StripTransientUserBlocks(content)
	s = HandoffTask(s)
	s = StripTransientUserBlocks(s)
	return strings.TrimSpace(s)
}

var pasteDisplayLabelPattern = regexp.MustCompile(`^\[(?:붙여넣은 텍스트|Pasted text) #[0-9]+ · [0-9]+ (?:줄|lines)\][ \t]*(?:\r?\n)?`)

func StripPasteDisplayLabel(content string) string {
	return pasteDisplayLabelPattern.ReplaceAllString(content, "")
}

func UserMessageText(msg provider.Message) string {
	if msg.RawContent != "" {
		return strings.TrimSpace(msg.RawContent)
	}
	return UserPreviewText(msg.Content)
}

func migrateLegacyProviderContent(msgs []provider.Message) []provider.Message {
	var upgraded []provider.Message
	for i, msg := range msgs {
		if msg.Role != provider.RoleUser {
			continue
		}
		switch {
		case msg.ProviderContent != "":
			if upgraded == nil {
				upgraded = append([]provider.Message(nil), msgs...)
			}
			if upgraded[i].RawContent == "" {
				upgraded[i].RawContent = msg.Content
			}
			upgraded[i].Content = msg.ProviderContent
			upgraded[i].ProviderContent = ""
		case msg.RawContent == "" && hasLegacyProviderWrapper(msg.Content):
			if upgraded == nil {
				upgraded = append([]provider.Message(nil), msgs...)
			}
			upgraded[i].RawContent = UserPreviewText(msg.Content)
		}
	}
	if upgraded != nil {
		return upgraded
	}
	return msgs
}

func hasLegacyProviderWrapper(content string) bool {
	if ContainsMemoryCompilerExecution(content) || reTransientUserBlock.MatchString(content) {
		return true
	}
	if stripTrailingDeliveryRuntime(content) != content {
		return true
	}
	stripped := StripTransientUserBlocks(content)
	return HandoffTask(stripped) != stripped
}

var SyntheticUserPrefixes = []string{
	"<reasoning-language>",
	"Plan approved — plan mode is off",
	"Host final-answer readiness check failed",
	"You are already in the executor phase",
	"The previous assistant response was interrupted while a tool call",
	"The previous assistant response was interrupted during streaming",
	"The previous assistant response was interrupted before visible",
	"The previous assistant response finished without any visible answer",
	"<compaction-summary>",
	"Summary of the later conversation (compacted from here on):",
	"Summary of earlier conversation (compacted up to here):",
	"Continue pursuing the active goal",
	"The agent signaled goal completion and all tasks are marked done.",
	"Goal signaled complete but issues remain:",
	"No tool calls in recent turns.",
}

func IsSyntheticUserText(content string) bool {
	trimmed := strings.TrimSpace(StripTransientUserBlocks(content))
	for _, prefix := range SyntheticUserPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func IsUserAuthoredTurn(content string) bool {
	if strings.TrimSpace(StripTransientUserBlocks(content)) == "" {
		return false
	}
	if IsSyntheticUserText(content) {
		return false
	}
	if _, isSteer := SteerText(content); isSteer {
		return false
	}
	return true
}
