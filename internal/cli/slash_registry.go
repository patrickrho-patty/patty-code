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
	hidden     bool
}

func builtinSlashSpecs() []builtinSlashSpec {
	specs := []builtinSlashSpec{
		{name: "/compact", ko: "/압축", insert: "/compact ", hint: i18n.M.CmdCompact, showInHelp: true},
		{name: "/new", ko: "/새세션", insert: "/new ", hint: i18n.M.CmdNew, showInHelp: true},
		{name: "/clear", ko: "/초기화", insert: "/clear", hint: i18n.M.CmdClear, showInHelp: true},
		{name: "/cls", ko: "/화면정리", insert: "/cls", hint: i18n.M.CmdCls, showInHelp: true},
		{name: "/resume", ko: "/작업재개", insert: "/resume ", hint: i18n.M.CmdResume, showInHelp: true},
		{name: "/rename", ko: "/이름변경", insert: "/rename ", hint: i18n.M.CmdRename, showInHelp: true},
		{name: "/rewind", ko: "/되감기", insert: "/rewind", hint: i18n.M.CmdRewind, showInHelp: true},
		{name: "/tree", ko: "/브랜치트리", insert: "/tree", hint: i18n.M.CmdTree, showInHelp: true},
		{name: "/branch", ko: "/새브랜치", insert: "/branch ", hint: i18n.M.CmdBranch, showInHelp: true},
		{name: "/switch", ko: "/브랜치전환", insert: "/switch ", hint: i18n.M.CmdSwitchBranch, showInHelp: true},
		{name: "/todo", ko: "/작업목록", insert: "/todo", hint: i18n.M.CmdTodo, showInHelp: true},
		{name: "/mcp", ko: "/mcp서버", insert: "/mcp", hint: i18n.M.CmdMcp, showInHelp: true},
		{name: "/remote", ko: "/원격호스트", insert: "/remote", hint: i18n.M.CmdRemote, showInHelp: true, hidden: true},
		{name: "/plugins", ko: "/플러그인", aliases: []string{"/plugin"}, insert: "/plugins", hint: i18n.M.CmdPlugins, showInHelp: true},
		{name: "/model", ko: "/모델변경", insert: "/model", hint: i18n.M.CmdModel, descend: true, showInHelp: true},
		{name: "/status", ko: "/현재상태", insert: "/status", hint: i18n.M.CmdStatus, showInHelp: true},
		{name: "/provider", ko: "/공급자변경", insert: "/provider", hint: i18n.M.CmdProvider, descend: true, showInHelp: true, hidden: true},
		{name: "/skills", ko: "/스킬설정", aliases: []string{"/skill"}, insert: "/skills", hint: i18n.M.CmdSkill, showInHelp: true},
		{name: "/compliance", ko: "/컴플라이언스", aliases: []string{"/pipa", "/kisa", "/csap"}, insert: "/compliance ", hint: i18n.M.CmdCompliance, showInHelp: true},
		{name: "/reload-cmd", ko: "/명령어갱신", insert: "/reload-cmd", hint: i18n.M.CmdReloadCmd, showInHelp: true, hidden: true},
		{name: "/reload", ko: "/새로고침", insert: "/reload", hint: i18n.M.CmdReload, showInHelp: true},
		{name: "/hooks", ko: "/훅설정", insert: "/hooks ", hint: i18n.M.CmdHooks, descend: true, showInHelp: true},
		{name: "/paste-image", ko: "/이미지첨부", insert: "/paste-image", hint: i18n.M.CmdPasteImage},
		{name: "/output-style", ko: "/출력스타일", aliases: []string{"/output-styles"}, insert: "/output-style", hint: i18n.M.CmdOutputStyle, showInHelp: true},
		{name: "/verbose", ko: "/생각표시", insert: "/verbose", hint: i18n.M.CmdVerbose, showInHelp: true},
		{name: "/mouse", ko: "/마우스", insert: "/mouse", hint: i18n.M.CmdMouse, showInHelp: true, hidden: true},
		{name: "/diff-fold", ko: "/비교접기", insert: "/diff-fold", hint: i18n.M.CmdDiffFold, showInHelp: true},
		{name: "/sandbox", ko: "/샌드박스", insert: "/sandbox", hint: i18n.M.CmdSandbox, showInHelp: true},
		{name: "/effort", ko: "/추론강도", insert: "/effort ", hint: i18n.M.CmdEffort, descend: true},
		{name: "/theme", ko: "/테마변경", insert: "/theme ", hint: i18n.M.CmdTheme, descend: true},
		{name: "/language", ko: "/언어설정", insert: "/language ", hint: i18n.M.CmdLanguage, descend: true, showInHelp: true},
		{name: "/help", ko: "/도움말", insert: "/help", hint: i18n.M.CmdHelp, hidden: true},
		{name: "/docs", ko: "/문서검색", aliases: []string{"/patty:docs"}, insert: "/docs ", hint: i18n.M.CmdDocs, showInHelp: true, hidden: true},
		{name: "/memory", ko: "/메모리", insert: "/memory ", hint: i18n.M.CmdMemory, showInHelp: true},
		{name: "/migrate", ko: "/마이그레이션", aliases: []string{"/migration"}, insert: "/migrate", hint: i18n.M.CmdMigrate, showInHelp: true, hidden: true},
		{name: "/goal", ko: "/목표", insert: "/goal ", hint: i18n.M.CmdGoal, descend: true},
		{name: "/remember", ko: "/기억추가", insert: "/remember ", hint: i18n.M.CmdRemember},
		{name: "/forget", ko: "/기억삭제", insert: "/forget ", hint: i18n.M.CmdForget},
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
// e.g. "/모델변경" → "/ㅁㄷㅂㄱ". Non-Hangul runes pass through unchanged so a
// "/" prefix and any Latin command suffix survive.
func chosungOf(s string) string {
	var b strings.Builder
	for _, r := range s {
		if leading := hangulLeadingJamo(r); leading != 0 {
			b.WriteRune(leading)
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
