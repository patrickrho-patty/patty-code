package taskintent

import "testing"

func TestDeliveryClassificationMatrix(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantEvidence bool
		wantMutation bool
	}{
		{"make sense", "why does this make sense?", false, false},
		{"node prose", "why does the node selection matter?", false, false},
		{"swift prose", "why does Swift concurrency work this way?", false, false},
		{"go prose", "why does this go wrong so often?", false, false},
		{"remote url", "why can't I open https://github.com?", false, false},
		{"remote analysis", "can you analyze why Outlook won't sync?", false, false},
		{"how to install", "how do I install the plugin?", false, false},
		{"korean how to install", "플러그인을 어떻게 설치하나요", false, false},
		{"korean how to modify", "에디터 설정을 어떻게 수정하나요", false, false},
		{"python prose", "why is Python popular?", false, false},
		{"docker prose", "why is Docker popular?", false, false},
		{"pytest prose", "why is pytest popular?", false, false},
		{"backtick python prose", "why is `Python` popular?", false, false},
		{"backtick code identifier", "what does `context.Context` mean?", false, false},
		{"double dash prose", "why is this -- strangely -- happening?", false, false},
		{"advisory upgrade", "왜 플러그인을 업그레이드한 후 실패했을까", false, false},
		{"advisory adjustment", "왜 설정을 조정한 후 효과가 없을까", false, false},
		{"advisory change application", "왜 수정 신청이 실패했을까", false, false},
		{"negated change", "please don't make the requested changes", false, false},
		{"negated korean", "코드를 고치지 마세요", false, false},
		{"negated inspection", "Explain why it fails. Do not inspect or change files.", false, false},
		{"negated korean inspection", "왜 실패했는지 설명해 주고, 파일은 건드리지 마세요", false, false},
		{"review only", "review only and do not fix anything", true, false},
		{"markdown anchor", "why does README.md render incorrectly?", true, false},
		{"git command", "why does git diff --check fail?", true, false},
		{"custom command", "why does mytool --strict fail?", true, false},
		{"repository anchor", "can you analyze this repository and explain why it fails?", true, false},
		{"observable audit", "audit the current configuration", true, false},
		{"make changes", "make the necessary changes", true, true},
		{"thanks then update", "thanks for fixing that, now update the tests", true, true},
		{"korean thanks then update", "고마워요, 계속해서 설정을 수정해 줘", true, true},
		{"thanks then review", "thanks for the help; now review this pull request", true, false},
		{"modify", "please modify the config", true, true},
		{"push", "push the branch", true, true},
		{"commit", "commit the fix", true, true},
		{"move", "move the file", true, true},
		{"bump", "bump the dependency", true, true},
		{"advice then fix", "why does it fail and fix it", true, true},
		{"polite fix before advice", "could you please fix why it fails", true, true},
		{"korean polite fix before advice", "왜 실패했는지, 수정해 줘", true, true},
		{"korean advice then fix", "왜 실패했는지, 수정해 줘", true, true},
		{"negated install then patch", "I cannot install dependencies but patch the parser", true, true},
		{"shared negation", "I cannot install and update dependencies", false, false},
		{"reset negation", "I cannot install dependencies and please update config", true, true},
		{"korean reset", "의존성을 설치할 수 없어서, 기존 설정에서 수정해 줘", true, true},
		{"negated request", "팀이 코드를 고치길 원하지 않아", false, false},
		{"negated application", "설정 고치는 신청은 안 돼", false, false},
		{"deferred conversation token", "Remember ORBIT-42 and answer on the next turn.", false, false},
		{"korean deferred conversation token", "ORBIT-42를 기억하고, 다음 라운드에서 답변해 줘", false, false},
		{"deferred token with durable negation", "Remember ORBIT-42 for the next turn. Do not save it permanently.", false, false},
		{"korean deferred token with durable negation", "ORBIT-42를 기억하고, 다음 라운드에서 답변해 줘. 파일이나 장기 기억에 기록하지 마세요.", false, false},
		{"long conversational context", "Please keep this code in mind because I will ask you about it in my next message", false, false},
		{"durable memory", "Remember ORBIT-42 permanently across sessions", true, true},
		{"korean durable memory", "ORBIT-42를 영구히 기억해요, 세션 간에도 보관해 줘", true, true},
		{"durable memory advice", "How do I save a preference permanently across sessions?", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsEvidence(tt.input); got != tt.wantEvidence {
				t.Errorf("NeedsEvidence(%q) = %v, want %v", tt.input, got, tt.wantEvidence)
			}
			if got := NeedsMutation(tt.input); got != tt.wantMutation {
				t.Errorf("NeedsMutation(%q) = %v, want %v", tt.input, got, tt.wantMutation)
			}
		})
	}
}
