package i18n

import (
	"reflect"
	"strings"
	"testing"
)

func TestCatalogsComplete(t *testing.T) {
	baseline := reflect.ValueOf(Korean)
	typ := baseline.Type()
	catalogs := map[string]reflect.Value{"ko": reflect.ValueOf(Korean), "en": reflect.ValueOf(English)}
	for tag, cat := range catalogs {
		for i := range typ.NumField() {
			name := typ.Field(i).Name
			if strings.TrimSpace(cat.Field(i).String()) == "" {
				t.Errorf("%s catalogue: field %q is empty", tag, name)
			}
		}
	}
}

// gain %s/%d/%q placeholders — a class of bug that only blows up when the
// languages for any field whose name ends in "Fmt".
func TestCatalogsAgreeOnPlaceholders(t *testing.T) {
	ko := reflect.ValueOf(Korean)
	en := reflect.ValueOf(English)
	typ := ko.Type()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if !strings.HasSuffix(name, "Fmt") {
			continue
		}
		want := countVerbs(ko.Field(i).String())
		got := countVerbs(en.Field(i).String())
		if want != got {
			t.Errorf("%s: ko has %d verbs, en has %d", name, want, got)
		}
	}
}

func TestKoreanCatalogContainsNoHanCharacters(t *testing.T) {
	ko := reflect.ValueOf(Korean)
	typ := ko.Type()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		value := ko.Field(i).String()
		for _, r := range value {
			if r >= '\u4e00' && r <= '\u9fff' {
				t.Fatalf("%s contains Han character %q in %q", name, r, value)
			}
		}
	}
}

func TestPlanApprovalChoicesExposeThreeExplicitActions(t *testing.T) {
	tests := []struct {
		tag   string
		value string
		want  []string
	}{
		{tag: "en", value: English.PlanApprovalChoices, want: []string{"Start execution", "Revise plan", "Exit without executing"}},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			numbered := 0
			for line := range strings.SplitSeq(tt.value, "\n") {
				line = strings.TrimSpace(line)
				if len(line) >= 3 && line[0] >= '1' && line[0] <= '9' && line[1] == '.' {
					numbered++
				}
			}
			if numbered != 3 {
				t.Fatalf("numbered Plan actions = %d, want 3:\n%s", numbered, tt.value)
			}
			for _, want := range tt.want {
				if !strings.Contains(tt.value, want) {
					t.Errorf("Plan choices missing %q:\n%s", want, tt.value)
				}
			}
		})
	}
}

// countVerbs counts unescaped fmt placeholders (%s, %d, %q, %v, …). %% does
func countVerbs(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			i++
			continue
		}
		n++
	}
	return n
}

// TestNormalize covers the locale-string shapes likely to land in $LANG /
// $LC_ALL / $PATTY_LANG. Unknown locales return "" so DetectLanguage falls
func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"en":              "en",
		"en_US.UTF-8":     "en",
		"ko-KR":           "ko",
		"ko_KR.UTF-8":     "ko",
		"한국어":             "ko",
		"Korean":          "ko",
		"en-US":           "en",
		"zh_CN.UTF-8":     "", // Chinese is no longer a supported option
		"zh-Hans-CN":      "",
		"Chinese (China)": "",
		"중국어":             "",
		"zh_TW.UTF-8":     "",
		"zh-Hant-TW":      "",
		"zh-Hant":         "",
		"번체":              "",
		"fr_FR.UTF-8":     "",
		"  ZH_TW  ":       "",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectLanguagePriority(t *testing.T) {
	t.Setenv("PATTY_LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	defer DetectLanguage("") // restore default for other tests

	if got := DetectLanguage(""); got != "ko" {
		t.Errorf("clean env: got %q, want ko (Korean default)", got)
	}

	t.Setenv("LANG", "en_US.UTF-8")
	if got := DetectLanguage(""); got != "ko" {
		t.Errorf("LANG=en_US.UTF-8: got %q, want ko (ambient locale does not override product default)", got)
	}

	t.Setenv("PATTY_LANG", "en")
	if got := DetectLanguage(""); got != "en" {
		t.Errorf("PATTY_LANG=en: got %q, want explicit English opt-in", got)
	}

	if got := DetectLanguage("en-US"); got != "en" {
		t.Errorf("override=en-US: got %q, want en", got)
	}
	if got := CurrentLanguage(); got != "en" {
		t.Errorf("current language = %q, want en", got)
	}
	if got := DetectLanguage("ko-KR"); got != "ko" || CurrentLanguage() != "ko" {
		t.Errorf("korean current language = %q/%q, want ko", got, CurrentLanguage())
	}
}

func TestSeoulFlowChromeCopyIsCompleteInKoreanAndEnglish(t *testing.T) {
	t.Cleanup(func() { DetectLanguage("") })

	for _, tt := range []struct {
		name string
		lang string
		want []string
	}{
		{
			name: "korean-default",
			lang: "ko",
			want: []string{
				"명령 / 메시지", "명령 또는 질문을 입력해보세요",
				"/ 명령어", "@ 파일", "! 셸", "? 단축키",
				"작업", "모델", "추론", "여유", "자동", "보통",
			},
		},
		{
			name: "english-selectable",
			lang: "en",
			want: []string{
				"COMMAND / MESSAGE", "Type a command or ask a question",
				"/ commands", "@ files", "! shell", "? shortcuts",
				"MODE", "MODEL", "EFFORT", "HEADROOM", "AUTO", "MEDIUM",
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			DetectLanguage(tt.lang)
			got := []string{
				M.ChatComposerTitle,
				M.ChatComposerPlaceholder,
				M.ChatComposerCommandsHint,
				M.ChatComposerFilesHint,
				M.ChatComposerShellHint,
				M.ChatComposerShortcutsHint,
				M.ChatStatusModeLabel,
				M.ChatStatusModelLabel,
				M.ChatStatusEffortLabel,
				M.ChatStatusHeadroomLabel,
				M.ChatModeAuto,
				M.ChatEffortMedium,
			}
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Fatalf("Seoul-flow chrome copy = %#v, want %#v", got, tt.want)
			}
			if strings.Contains(strings.Join(got, " "), "ENGINE") {
				t.Fatalf("Seoul-flow model label must be MODEL, not ENGINE: %#v", got)
			}
		})
	}
}
