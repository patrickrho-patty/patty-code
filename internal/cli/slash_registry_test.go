package cli

import "testing"

func TestBuiltinSlashRegistryHasUniqueNamesAndAliases(t *testing.T) {
	seen := map[string]string{}
	for _, spec := range builtinSlashSpecs() {
		if spec.name == "" || spec.name[0] != '/' {
			t.Fatalf("invalid built-in slash name %q", spec.name)
		}
		if spec.ko == "" || spec.ko[0] != '/' {
			t.Fatalf("invalid built-in Korean name %q for %q", spec.ko, spec.name)
		}
		// name, aliases, Korean canonical name, and 초성 must each resolve to
		// exactly one command and never collide with another command's surface.
		for _, name := range append([]string{spec.name, spec.ko, spec.chosung}, spec.aliases...) {
			if name == "" {
				continue
			}
			if owner, exists := seen[name]; exists {
				t.Fatalf("slash name %q belongs to both %q and %q", name, owner, spec.name)
			}
			seen[name] = spec.name
			if got := canonicalBuiltinSlashCommand(name); got != spec.name {
				t.Fatalf("canonical command for %q = %q, want %q", name, got, spec.name)
			}
		}
	}
	// 초성 must be derivable for every Korean name so the palette can advertise it.
	for _, spec := range builtinSlashSpecs() {
		if spec.chosung == "" {
			t.Fatalf("초성 for %q (%q) is empty", spec.name, spec.ko)
		}
	}
}

func TestBuiltinSlashCompletionAndHelpComeFromRegistry(t *testing.T) {
	specs := builtinSlashSpecs()
	completion := builtinSlashItems()
	if len(completion) != len(specs) {
		t.Fatalf("completion items = %d, specs = %d", len(completion), len(specs))
	}
	help := builtinSlashHelpItems()
	helpNames := map[string]bool{}
	for _, item := range help {
		helpNames[item.label] = true
	}
	for i, spec := range specs {
		item := completion[i]
		if item.label != slashDisplayName(spec) || item.hint != spec.hint || item.descend != spec.descend {
			t.Fatalf("completion item for %q drifted: %+v", spec.name, item)
		}
		if helpNames[slashDisplayName(spec)] != spec.showInHelp {
			t.Fatalf("help visibility for %q = %v, want %v", spec.name, helpNames[slashDisplayName(spec)], spec.showInHelp)
		}
	}
}

func TestCanonicalSlashCommandResolvesChosungUniquely(t *testing.T) {
	specs := builtinSlashSpecs()
	// An exact 초성 match must resolve.
	for _, spec := range specs {
		if got := canonicalBuiltinSlashCommand(spec.chosung); got != spec.name {
			t.Fatalf("초성 %q resolves to %q, want %q", spec.chosung, got, spec.name)
		}
	}
	// A one-jamo prefix shared by several commands must NOT resolve (opaque),
	// so the palette can ask for more input instead of guessing.
	ambiguous := map[string]int{}
	for _, spec := range specs {
		oneJamo := spec.chosung
		if len([]rune(oneJamo)) > 1 {
			oneJamo = string([]rune(oneJamo)[:1])
		}
		ambiguous[oneJamo]++
	}
	for jamo, count := range ambiguous {
		if count > 1 {
			if got := canonicalBuiltinSlashCommand(jamo); got != jamo {
				t.Fatalf("ambiguous 초성 %q resolved to %q, want it left opaque (count=%d)", jamo, got, count)
			}
		}
	}
}