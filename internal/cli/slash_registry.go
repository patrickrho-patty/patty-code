package cli

import (
	"slices"
	"strings"

	"patty/internal/i18n"
)

// builtinSlashSpec is the single source of truth for a built-in command's
// discoverable surface. name is the stable English dispatch key; ko is the
// Korean canonical name; chosung is the Korean initial-consonant (초성) alias.
type builtinSlashSpec struct {
	name       string
	ko         string
	chosung    string
	aliases    []string
	insert     string
	hint       string
	descend    bool
	showInHelp bool
}

func builtinSlashSpecs() []builtinSlashSpec {
	specs := []builtinSlashSpec{
		{name: "/compact", ko: "/압축", insert: "/compact ", hint: i18n.M.CmdCompact, showInHelp: true},
		{name: "/new", ko: "/새세션", insert: "/new ", hint: i18n.M.CmdNew, showInHelp: true},
		{name: "/clear", ko: "/지우기", insert: "/clear", hint: i18n.M.CmdClear, showInHelp: true},
		{name: "/cls", ko: "/화면지우기", insert: "/cls", hint: i18n.M.CmdCls, showInHelp: true},
		{name: "/resume", ko: "/이어하기", insert: "/resume ", hint: i18n.M.CmdResume, showInHelp: true},
		{name: "/rename", ko: "/이름바꾸기", insert: "/rename ", hint: i18n.M.CmdRename, showInHelp: true},
		{name: "/rewind", ko: "/되감기", insert: "/rewind", hint: i18n.M.CmdRewind, showInHelp: true},
		{name: "/tree", ko: "/브랜치보기", insert: "/tree", hint: i18n.M.CmdTree, showInHelp: true},
		{name: "/branch", ko: "/브랜치만들기", insert: "/branch ", hint: i18n.M.CmdBranch, showInHelp: true},
		{name: "/switch", ko: "/브랜치전환", insert: "/switch ", hint: i18n.M.CmdSwitchBranch, showInHelp: true},
		{name: "/todo", ko: "/작업목록", insert: "/todo", hint: i18n.M.CmdTodo, showInHelp: true},
		{name: "/mcp", ko: "/서버관리", insert: "/mcp", hint: i18n.M.CmdMcp, showInHelp: true},
		{name: "/remote", ko: "/원격호스트", insert: "/remote", hint: i18n.M.CmdRemote, showInHelp: true},
		{name: "/plugins", ko: "/플러그인", aliases: []string{"/plugin"}, insert: "/plugins", hint: i18n.M.CmdPlugins, showInHelp: true},
		{name: "/model", ko: "/모델전환", insert: "/model", hint: i18n.M.CmdModel, descend: true, showInHelp: true},
		{name: "/status", ko: "/상태보기", insert: "/status", hint: i18n.M.CmdStatus, showInHelp: true},
		{name: "/work-mode", ko: "/작업모드", aliases: []string{"/profile"}, insert: "/work-mode ", hint: i18n.M.CmdWorkMode, descend: true, showInHelp: false},
		{name: "/provider", ko: "/공급자전환", insert: "/provider", hint: i18n.M.CmdProvider, descend: true, showInHelp: true},
		{name: "/skills", ko: "/스킬관리", aliases: []string{"/skill"}, insert: "/skills", hint: i18n.M.CmdSkill, showInHelp: true},
		{name: "/reload-cmd", ko: "/명령새로고침", insert: "/reload-cmd", hint: i18n.M.CmdReloadCmd, showInHelp: true},
		{name: "/reload", ko: "/런타임새로고침", insert: "/reload", hint: i18n.M.CmdReload, showInHelp: true},
		{name: "/hooks", ko: "/훅관리", insert: "/hooks ", hint: i18n.M.CmdHooks, descend: true, showInHelp: true},
		{name: "/paste-image", ko: "/이미지붙여넣기", insert: "/paste-image", hint: i18n.M.CmdPasteImage},
		{name: "/output-style", ko: "/출력스타일", aliases: []string{"/output-styles"}, insert: "/output-style", hint: i18n.M.CmdOutputStyle, showInHelp: true},
		{name: "/verbose", ko: "/생각표시", insert: "/verbose", hint: i18n.M.CmdVerbose, showInHelp: true},
		{name: "/mouse", ko: "/마우스", insert: "/mouse", hint: i18n.M.CmdMouse, showInHelp: true},
		{name: "/diff-fold", ko: "/차이접기", insert: "/diff-fold", hint: i18n.M.CmdDiffFold, showInHelp: true},
		{name: "/sandbox", ko: "/샌드박스", insert: "/sandbox", hint: i18n.M.CmdSandbox, showInHelp: true},
		{name: "/effort", ko: "/추론강도", insert: "/effort ", hint: i18n.M.CmdEffort, descend: true},
		{name: "/reasoning-language", ko: "/추론언어", insert: "/reasoning-language ", hint: i18n.M.CmdReasonLang, descend: true, showInHelp: true},
		{name: "/theme", ko: "/테마전환", insert: "/theme ", hint: i18n.M.CmdTheme, descend: true},
		{name: "/language", ko: "/언어설정", insert: "/language ", hint: i18n.M.CmdLanguage, descend: true, showInHelp: true},
		{name: "/currency", ko: "/통화설정", insert: "/currency ", hint: i18n.M.CmdCurrency, descend: true, showInHelp: true},
		{name: "/help", ko: "/도움말", insert: "/help", hint: i18n.M.CmdHelp, showInHelp: true},
		{name: "/docs", ko: "/문서검색", aliases: []string{"/patty:docs"}, insert: "/docs ", hint: i18n.M.CmdDocs, showInHelp: true},
		{name: "/memory", ko: "/메모리", insert: "/memory ", hint: i18n.M.CmdMemory, showInHelp: true},
		{name: "/migrate", ko: "/마이그레이션", aliases: []string{"/migration"}, insert: "/migrate", hint: i18n.M.CmdMigrate, showInHelp: true},
		{name: "/goal", ko: "/목표", insert: "/goal ", hint: i18n.M.CmdGoal, descend: true},
		{name: "/remember", ko: "/기억하기", insert: "/remember ", hint: i18n.M.CmdRemember},
		{name: "/forget", ko: "/잊기", insert: "/forget ", hint: i18n.M.CmdForget},
		{name: "/quit", ko: "/종료", aliases: []string{"/exit"}, insert: "/quit", hint: i18n.M.CmdQuit},
		{name: "/copy", ko: "/복사", insert: "/copy", hint: i18n.M.CmdCopy, showInHelp: true},
		{name: "/export", ko: "/내보내기", insert: "/export", hint: i18n.M.CmdExport, showInHelp: true},
	}
	populateChosung(specs)
	return specs
}

// populateChosung derives each spec's 초성 alias from its Korean name.
func populateChosung(specs []builtinSlashSpec) {
	for i := range specs {
		specs[i].chosung = chosungOf(specs[i].ko)
	}
}

// chosungOf maps a Hangul syllable string to its leading-jamo (초성) spelling,
// e.g. "/이어하기" → "/ㅇㅇㅎㄱ". Non-Hangul runes pass through unchanged so a
// "/" prefix and any Latin command suffix survive.
func chosungOf(s string) string {
	cho := []rune("ㄱㄲㄴㄷㄸㄹㅁㅂㅃㅅㅆㅇㅈㅉㅊㅋㅌㅍㅎ")
	var b strings.Builder
	for _, r := range s {
		if r >= 0xAC00 && r <= 0xD7A3 {
			b.WriteRune(cho[(r-0xAC00)/(21*28)])
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// slashDisplayName returns the label shown in the menu for the active locale:
// the Korean canonical name with its English counterpart in parentheses when
// Korean is selected (so Korean users who know harness conventions can type the
// English form), the English name otherwise.
func slashDisplayName(spec builtinSlashSpec) string {
	if i18n.CurrentLanguage() == "ko" && spec.ko != "" {
		if en := strings.TrimPrefix(spec.name, "/"); en != "" && en != strings.TrimPrefix(spec.ko, "/") {
			return spec.ko + " (" + en + ")"
		}
		return spec.ko
	}
	return spec.name
}

func builtinSlashItems() []compItem {
	specs := builtinSlashSpecs()
	populateChosung(specs)
	items := make([]compItem, 0, len(specs))
	for _, spec := range specs {
		label := slashDisplayName(spec)
		insert := spec.insert
		if i18n.CurrentLanguage() == "ko" && spec.ko != "" {
			insert = spec.ko + strings.TrimPrefix(spec.insert, spec.name)
		}
		items = append(items, compItem{
			label: label, insert: insert, hint: spec.hint, descend: spec.descend,
			aliases: []string{spec.ko, spec.chosung, spec.name},
		})
	}
	return items
}

func builtinSlashHelpItems() []compItem {
	specs := builtinSlashSpecs()
	populateChosung(specs)
	var items []compItem
	for _, spec := range specs {
		if spec.showInHelp {
			items = append(items, compItem{label: slashDisplayName(spec), hint: spec.hint, aliases: []string{spec.ko, spec.chosung, spec.name}})
		}
	}
	return items
}

func canonicalBuiltinSlashCommand(name string) string {
	name = strings.TrimSpace(name)
	specs := builtinSlashSpecs()
	populateChosung(specs)
	for _, spec := range specs {
		if name == spec.name || name == spec.ko || slices.Contains(spec.aliases, name) {
			return spec.name
		}
	}
	// 초성 input resolves only when exactly one command matches; an ambiguous
	// prefix stays unresolved so the palette can ask for more characters
	// (spec §11.2 — ambiguous 초성 never auto-executes). An exact 초성 match
	// wins over a shared prefix ("/ㅇㄱ" is exactly /forget even though it also
	// prefixes /원격호스트).
	var exact string
	for _, spec := range specs {
		if spec.chosung == "" || spec.chosung != name {
			continue
		}
		if exact != "" {
			return name
		}
		exact = spec.name
	}
	if exact != "" {
		return exact
	}
	var prefixed string
	for _, spec := range specs {
		if spec.chosung == "" || !strings.HasPrefix(spec.chosung, name) {
			continue
		}
		if prefixed != "" {
			return name
		}
		prefixed = spec.name
	}
	if prefixed != "" {
		return prefixed
	}
	return name
}
