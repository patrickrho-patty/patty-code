package taskintent

import "testing"

// TestHeuristicInputIsTask covers the delivery evidence gates task-vs-chat
func TestHeuristicInputIsTask(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Greetings / acknowledgements — chat.
		{"hello", "hello", false},
		{"hi", "hi", false},
		{"안녕하세요", "안녕하세요", false},
		{"thanks", "thanks", false},
		{"고마워요", "고마워요", false},
		{"ok", "ok", false},
		{"좋습니다", "좋습니다", false},
		{"확인", "확인", false},
		{"알겠습니다", "알겠습니다", false},
		{"일단 괜찮아요", "일단 괜찮아요", false},

		{"fix bug", "fix the bug", true},
		{"create component", "create a component", true},
		{"문제 수정", "이 문제를 수정해 줘", true},
		{"run tests", "run tests", true},
		{"modify config", "modify config", true},
		{"make changes", "make the requested changes", true},
		{"설정 조정", "기존 설정을 조정해 줘", true},
		{"push branch", "push the branch", true},
		{"publish release", "publish release", true},
		{"봐주세요", "이 오류 좀 봐주세요", true},
		{"한번 봐주세요", "이 문제 좀 한번 봐주세요", true},
		{"처리해 주세요", "이 issue 좀 처리해 주세요", true},
		{"조사", "시작 실패를 조사해 줘", true},
		{"위치 파악", "이 이상 현상의 위치를 파악해 줘", true},

		{"thanks for fixing", "thanks for fixing that!", false},
		{"check later", "I'll check later", false},
		{"test later", "I'll test it later", false},
		{"test was helpful", "that test was helpful", false},
		{"수고하셨습니다", "수고하셨습니다", false},
		{"thanks then update", "thanks for fixing that, now update the tests", true},
		{"korean thanks then update", "고마워요, 계속해서 설정을 수정해 줘", true},
		{"task before thanks", "review this PR; thanks for the help", true},

		{"auth not working", "the auth isn't working", true},
		{"help with login", "can you help with login?", true},
		{"문제 심각", "이 문제는 아주 심각해", true},
		{"멈췄어요", "페이지가 멈췄어요", true},
		{"반응 없음", "버튼을 눌러도 반응 없음", true},
		{"적용 안 됨", "설정이 적용 안 됨", true},
		{"비정상 종료", "프로그램이 비정상 종료됨", true},

		{"file reference", "what about @auth.go", true},
		{"localized file reference", "（@설정.yaml）를 검사해 줘", true},
		{"python file", "check main.py", true},
		{"markdown file", "why does README.md render incorrectly", true},
		{"relative script", "why does ./scripts/verify.sh fail", true},
		{"email is not file reference", "thanks, email me@example.com later", false},
		{"long conversation is not task", "Remember ORBIT-42 and answer on the next turn please", false},
		{"durable memory is task", "Remember ORBIT-42 permanently across sessions", true},

		{"empty", "", false},
		{"spaces", "   ", false},
		{"question mark", "?", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := heuristicInputIsTask(tt.input); got != tt.want {
				t.Errorf("heuristicInputIsTask(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
