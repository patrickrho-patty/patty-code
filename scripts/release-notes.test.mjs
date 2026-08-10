import assert from "node:assert/strict";
import { test } from "node:test";
import { loadCatalog, releaseForVersion, renderGitHubRelease, validateCatalog } from "./release-notes.mjs";
import { validateReleaseEvent } from "./release-event.mjs";

test("the committed release catalog is valid and newest first", async () => {
  const catalog = await loadCatalog();
  assert.equal(catalog.schemaVersion, 1);
  assert.equal(new Set(catalog.releases.map((release) => release.version)).size, catalog.releases.length);
});

test("tag namespaces resolve to the same product release", async () => {
  const catalog = await loadCatalog();
  assert.equal(releaseForVersion(catalog, "v1.17.13").version, "1.17.13");
  assert.equal(releaseForVersion(catalog, "desktop-v1.17.13").version, "1.17.13");
  assert.equal(releaseForVersion(catalog, "npm-v1.17.13").version, "1.17.13");
});

test("GitHub rendering keeps product sections and source PR links", async () => {
  const catalog = await loadCatalog();
  const markdown = renderGitHubRelease(releaseForVersion(catalog, "1.17.13"), "ko-KR");
  assert.match(markdown, /## 사용 가이드/);
  assert.match(markdown, /## 주요 내용/);
  assert.match(markdown, /## 업그레이드 안내/);
  assert.match(markdown, /## 위험 안내/);
  assert.match(markdown, /## 감사/);
  assert.match(markdown, /\/pull\/6460/);
  assert.match(markdown, /patty-code.io\/changelog\/v1\.17\.13/);
});

test("validation rejects unsupported locale fields", () => {
  assert.throws(
    () =>
      validateCatalog({
        schemaVersion: 1,
        releases: [
          {
            version: "1.0.0",
            date: "2026-01-01",
            channel: "stable",
            title: { en: "Title", extra: "제목" },
            summary: { en: "Summary", extra: "요약" },
          },
        ],
      }),
    /title\.extra/,
  );
});

test("managed Preview records bind every surface to one exact ordinal", () => {
  const release = {
    version: "1.19.0-preview.3",
    releaseId: "1.19.0-preview.3",
    baseVersion: "1.19.0",
    date: "2026-07-31",
    channel: "prerelease",
    status: "reviewed",
    previousRelease: "1.19.0-preview.2",
    builds: {
      cli: "v1.19.0-preview.3",
      desktop: "v1.19.0-preview.3",
      npm: "1.19.0-canary.3",
    },
    title: { en: "Preview", "ko-KR": "프리뷰" },
    summary: { en: "Preview summary", "ko-KR": "프리뷰 요약" },
    surfaces: ["cli"],
    guides: [],
    highlights: [{
      kind: "fixed",
      title: { en: "Fix", "ko-KR": "수정" },
      body: { en: "A fix.", "ko-KR": "수정 사항입니다." },
      refs: [1],
    }],
    changes: { new: [], improved: [], fixed: [] },
    upgrade: [],
    risks: [],
    contributors: [],
    links: {
      github: "https://github.com/pattycorp/DeepSeek-Patty Code/releases/tag/v1.19.0-preview.3",
      compare: "https://github.com/pattycorp/DeepSeek-Patty Code/compare/v1.19.0-preview.2...v1.19.0-preview.3",
      download: "https://patty-code.io/?download=desktop&channel=preview#start",
    },
  };

  assert.doesNotThrow(() => validateCatalog({ schemaVersion: 1, releases: [release] }));
  assert.throws(
    () => validateCatalog({
      schemaVersion: 1,
      releases: [{ ...release, builds: { ...release.builds, npm: "1.19.0-canary.4" } }],
    }),
    /builds\.npm/,
  );
});

test("publication marker is bound to reviewed version, channel, SHA, and builds", () => {
  const release = {
    version: "1.19.0-preview.3",
    channel: "prerelease",
    candidateSha: "a".repeat(40),
    builds: {
      cli: "v1.19.0-preview.3",
      desktop: "v1.19.0-preview.3",
      npm: "1.19.0-canary.3",
    },
  };
  const event = {
    schemaVersion: 1,
    releaseId: release.version,
    channel: "preview",
    candidateSha: release.candidateSha,
    publishedAt: "2026-07-31T00:00:00.000Z",
    releaseNotesUrl: "https://patty-code.io/changelog/v1.19.0-preview.3/",
    builds: release.builds,
  };

  assert.doesNotThrow(() => validateReleaseEvent(event, release));
  assert.throws(
    () => validateReleaseEvent({ ...event, candidateSha: "b".repeat(40) }, release),
    /candidateSha/,
  );
});
