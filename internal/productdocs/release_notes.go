package productdocs

import (
	"encoding/json"
	"fmt"
	"strings"
)

type localizedText struct {
	English string `json:"en"`
	Korean  string `json:"ko-KR"`
}

func (t localizedText) text(locale string) string {
	if locale == "ko-KR" {
		if k := strings.TrimSpace(t.Korean); k != "" {
			return k
		}
		return strings.TrimSpace(t.English)
	}
	return strings.TrimSpace(t.English)
}

type releaseItem struct {
	Kind  string        `json:"kind"`
	Level string        `json:"level"`
	Title localizedText `json:"title"`
	Body  localizedText `json:"body"`
	Href  string        `json:"href"`
	Refs  []int         `json:"refs"`
}

type releaseRecord struct {
	Version      string        `json:"version"`
	Date         string        `json:"date"`
	Channel      string        `json:"channel"`
	Status       string        `json:"status"`
	Title        localizedText `json:"title"`
	Summary      localizedText `json:"summary"`
	Surfaces     []string      `json:"surfaces"`
	Guides       []releaseItem `json:"guides"`
	Highlights   []releaseItem `json:"highlights"`
	Upgrade      []releaseItem `json:"upgrade"`
	Risks        []releaseItem `json:"risks"`
	Contributors []string      `json:"contributors"`
	Changes      struct {
		New      []releaseItem `json:"new"`
		Improved []releaseItem `json:"improved"`
		Fixed    []releaseItem `json:"fixed"`
	} `json:"changes"`
}

type renderedReleaseDocument struct {
	path    string
	source  string
	locale  string
	version string
	content string
}

func renderReleaseDocuments(data []byte) ([]renderedReleaseDocument, int, error) {
	var catalog struct {
		Releases []releaseRecord `json:"releases"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, 0, fmt.Errorf("decode release-notes/releases.json: %w", err)
	}
	if len(catalog.Releases) == 0 {
		return nil, 0, fmt.Errorf("release-notes/releases.json contains no releases")
	}
	seen := make(map[string]struct{}, len(catalog.Releases))
	documents := make([]renderedReleaseDocument, 0, len(catalog.Releases)*2)
	for _, release := range catalog.Releases {
		if !validReleaseVersion(release.Version) {
			return nil, 0, fmt.Errorf("release-notes/releases.json has invalid version %q", release.Version)
		}
		if _, duplicate := seen[release.Version]; duplicate {
			return nil, 0, fmt.Errorf("release-notes/releases.json has duplicate version %q", release.Version)
		}
		seen[release.Version] = struct{}{}
		for _, locale := range []string{"en", "ko-KR"} {
			suffix := ".md"
			if locale == "ko-KR" {
				suffix = ".ko-KR.md"
			}
			documents = append(documents, renderedReleaseDocument{
				path:    "changelog/v" + release.Version + suffix,
				source:  "release-notes/releases.json#v" + release.Version,
				locale:  locale,
				version: release.Version,
				content: renderReleaseMarkdown(release, locale),
			})
		}
	}
	return documents, len(catalog.Releases), nil
}

func validReleaseVersion(version string) bool {
	if version == "" || strings.Contains(version, "..") {
		return false
	}
	for _, r := range version {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func renderReleaseMarkdown(release releaseRecord, locale string) string {
	korean := locale == "ko-KR"
	localize := func(value localizedText) string {
		return value.text(locale)
	}
	heading := func(en, ko string) string {
		if korean {
			return ko
		}
		return en
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Patty Code v%s — %s\n\n", release.Version, localize(release.Title))
	if korean {
		fmt.Fprintf(&b, "배포일: %s\n\n배포 채널: %s\n\n상태: %s\n", release.Date, release.Channel, release.Status)
	} else {
		fmt.Fprintf(&b, "Release date: %s\n\nChannel: %s\n\nStatus: %s\n", release.Date, release.Channel, release.Status)
	}
	if len(release.Surfaces) > 0 {
		fmt.Fprintf(&b, "\n%s: %s\n", heading("Surfaces", "적용 제품 영역"), strings.Join(release.Surfaces, ", "))
	}

	fmt.Fprintf(&b, "\n## %s\n\n%s\n", heading("Summary", "요약"), localize(release.Summary))
	renderReleaseItems(&b, heading("Highlights", "주요 내용"), release.Highlights, localize, heading)
	if len(release.Changes.New)+len(release.Changes.Improved)+len(release.Changes.Fixed) > 0 {
		fmt.Fprintf(&b, "\n## %s\n", heading("Changes", "변경 사항"))
		renderReleaseItemGroup(&b, heading("New", "신규 추가"), release.Changes.New, localize, heading)
		renderReleaseItemGroup(&b, heading("Improved", "개선 사항"), release.Changes.Improved, localize, heading)
		renderReleaseItemGroup(&b, heading("Fixed", "복구"), release.Changes.Fixed, localize, heading)
	}
	renderReleaseItems(&b, heading("Upgrade notes", "액스레이제이션설명"), release.Upgrade, localize, heading)
	renderReleaseItems(&b, heading("Known risks", "알려진 위험"), release.Risks, localize, heading)
	renderReleaseItems(&b, heading("Related guides", "관련 가이드"), release.Guides, localize, heading)
	if len(release.Contributors) > 0 {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", heading("Contributors", "기여자"), strings.Join(release.Contributors, ", "))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func renderReleaseItems(
	b *strings.Builder,
	title string,
	items []releaseItem,
	localize func(localizedText) string,
	heading func(string, string) string,
) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n", title)
	renderReleaseItemBullets(b, items, localize, heading)
}

func renderReleaseItemGroup(
	b *strings.Builder,
	title string,
	items []releaseItem,
	localize func(localizedText) string,
	heading func(string, string) string,
) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n### %s\n", title)
	renderReleaseItemBullets(b, items, localize, heading)
}

func renderReleaseItemBullets(
	b *strings.Builder,
	items []releaseItem,
	localize func(localizedText) string,
	heading func(string, string) string,
) {
	for _, item := range items {
		title := localize(item.Title)
		body := localize(item.Body)
		if title == "" && body == "" {
			continue
		}
		fmt.Fprintf(b, "\n- **%s**", title)
		if item.Kind != "" {
			fmt.Fprintf(b, " [%s]", item.Kind)
		} else if item.Level != "" {
			fmt.Fprintf(b, " [%s]", item.Level)
		}
		if body != "" {
			fmt.Fprintf(b, ": %s", body)
		}
		if item.Href != "" {
			fmt.Fprintf(b, " [%s](%s)", heading("Open guide", "가이드 열기"), item.Href)
		}
		if len(item.Refs) > 0 {
			refs := make([]string, 0, len(item.Refs))
			for _, ref := range item.Refs {
				refs = append(refs, fmt.Sprintf("#%d", ref))
			}
			fmt.Fprintf(b, " (%s: %s)", heading("Refs", "참조"), strings.Join(refs, ", "))
		}
		b.WriteByte('\n')
	}
}
