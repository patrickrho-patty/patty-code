package cli

import (
	"testing"

	"patty/internal/skill"
)

func TestSkillPickerFilteredSkillsChosung(t *testing.T) {
	p := &skillPicker{
		skills: []skill.Skill{
			{Name: "스킬설정", Description: "manage skills", Scope: skill.ScopeProject, Path: "/fake/스킬설정/SKILL.md"},
			{Name: "review", Description: "Review code changes", Scope: skill.ScopeProject, Path: "/fake/review/SKILL.md"},
		},
	}

	// full chosung of 스킬설정 → ㅅㅋㅅㅈ
	p.query = "ㅅㅋㅅㅈ"
	if got := p.filteredSkills(); len(got) != 1 || got[0].Name != "스킬설정" {
		t.Fatalf("ㅅㅋㅅㅈ should match 스킬설정, got %+v", names(got))
	}

	// partial chosung prefix
	p.query = "ㅅㅋ"
	if got := p.filteredSkills(); len(got) != 1 || got[0].Name != "스킬설정" {
		t.Fatalf("ㅅㅋ should match 스킬설정, got %+v", names(got))
	}

	// jamo-free query: literal matching unchanged
	p.query = "review"
	if got := p.filteredSkills(); len(got) != 1 || got[0].Name != "review" {
		t.Fatalf("literal review should keep matching, got %+v", names(got))
	}

	// runes absent from every projection match nothing
	p.query = "ㄷㄷ"
	if got := p.filteredSkills(); len(got) != 0 {
		t.Fatalf("ㄷㄷ should match nothing, got %+v", names(got))
	}
}

func names(skills []skill.Skill) []string {
	out := make([]string, len(skills))
	for i, s := range skills {
		out[i] = s.Name
	}
	return out
}
