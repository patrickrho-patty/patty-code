"""Emit docs/THEME_ASSETS.md + .ko-KR.md — provenance & licence ledger for the
official theme assets. Reads hashes from out/report.json (produced by build.py).
"""
from __future__ import annotations

import datetime
import json
import os

HERE = os.path.dirname(__file__)
DOCS = os.path.join(HERE, "..", "..", "docs")

# scene function name + seed per theme (see scenes.py).
SCENES = {
    "official-rose-dawn": ("scene_rose_dawn", 20260117),
    "official-fortune-forge": ("scene_fortune_forge", 20260118),
    "official-crimson-horizon": ("scene_crimson_horizon", 20260119),
    "official-sage-breeze": ("scene_sage_breeze", 20260120),
    "official-spark-notebook": ("scene_spark_notebook", 20260121),
    "official-violet-starlight": ("scene_violet_starlight", 20260122),
    "official-cyan-stage": ("scene_cyan_stage", 20260123),
    "official-noir-gold": ("scene_noir_gold", 20260124),
}

NAMES = {
    "official-rose-dawn": "Rose Dawn / 장미 새벽",
    "official-fortune-forge": "Fortune Forge / 행운 공방",
    "official-crimson-horizon": "Crimson Horizon / 진홍 지평선",
    "official-sage-breeze": "Sage Breeze / 세이지 바람",
    "official-spark-notebook": "Spark Notebook / 스파크 노트북",
    "official-violet-starlight": "Violet Starlight / 바이올렛 별빛",
    "official-cyan-stage": "Cyan Stage / 시안 스테이지",
    "official-noir-gold": "Noir Gold / 누아르 골드",
}


def main():
    report = json.load(open(os.path.join(HERE, "out", "report.json")))
    today = datetime.date.today().isoformat()

    rows = []
    for tid in sorted(SCENES):
        fn, seed = SCENES[tid]
        r = report[tid]
        prompt = f"scripts/official-theme-art/scenes.py::{fn} (seed {seed}, 2560×1440 canvas, low-info left 0–52%, key content x 62–88% / y 16–72%)"
        rows.append((tid, NAMES[tid], prompt, r["background_sha256"], r["preview_sha256"]))

    en = []
    en.append("# Official Theme Asset Provenance\n")
    en.append("All eight official Patty Code themes ship with **original** artwork generated procedurally")
    en.append("from scratch with the scripts in `scripts/official-theme-art/` (numpy + Pillow, fixed seeds,")
    en.append("fully reproducible). The visual *direction* was inspired by the MIT-licensed")
    en.append("[Codex-Dream-Skin](https://github.com/Fei-Away/Codex-Dream-Skin) concept gallery, but:\n")
    en.append("- **No pixels, layouts, UI mockery, text, logos or watermarks were copied** from the reference")
    en.append("  project or any third party. Every background is re-authored code output.")
    en.append("- All depicted people are **original fictional adults** drawn by the generator: an illustrated")
    en.append("  muse (Rose Dawn), a lucky programmer mascot (Fortune Forge), a reader (Sage Breeze), an anime")
    en.append("  adult (Spark Notebook), a silhouette muse (Violet Starlight), a digital performer (Cyan Stage)")
    en.append("  and a gentleman (Noir Gold). Crimson Horizon contains no people.")
    en.append("- Backgrounds contain no windows, sidebars, cards, buttons, inputs or readable text, and are")
    en.append("  stripped of EXIF/author metadata.\n")
    en.append("Assets are released under the MIT License as part of the Patty Code repository,")
    en.append("© Patty Code Contributors. Human review: Patty Code Contributors (release PR review).\n")
    en.append(f"Generation date: {today}\n")
    en.append("| Theme | Generator (final prompt equivalent) | background.webp SHA-256 | preview.webp SHA-256 |")
    en.append("| --- | --- | --- | --- |")
    for tid, name, prompt, bg, pv in rows:
        en.append(f"| {name} (`{tid}`) | `{prompt}` | `{bg}` | `{pv}` |")
    en.append("")

    ko = []
    ko.append("# 공식 테마 에셋 출처 기록\n")
    ko.append("Patty Code의 공식 테마 8종은 모두 `scripts/official-theme-art/` 아래의 스크립트로")
    ko.append("처음부터 절차적으로 생성한 **오리지널** 이미지입니다. (numpy + Pillow, 고정 시드, 완전 재현 가능)")
    ko.append("시각적 *방향성*은 MIT 라이선스의")
    ko.append("[Codex-Dream-Skin](https://github.com/Fei-Away/Codex-Dream-Skin) 콘셉트 갤러리에서 영감을 받았지만, 다음을 보장합니다:\n")
    ko.append("- 참조 프로젝트나 제3자 에셋의 **픽셀, 레이아웃, UI 요소, 텍스트, 로고, 워터마크를 복제하지 않았습니다**.")
    ko.append("  모든 배경은 코드로 다시 생성한 독립 결과물입니다.")
    ko.append("- 등장 인물은 모두 생성기가 만든 **오리지널 성인 가상 인물**입니다: 일러스트 뮤즈(Rose Dawn),")
    ko.append("  행운의 프로그래머 마스코트(Fortune Forge), 독서하는 인물(Sage Breeze), 애니메이션풍 성인 인물(Spark Notebook),")
    ko.append("  실루엣 뮤즈(Violet Starlight), 디지털 퍼포머(Cyan Stage), 신사(Noir Gold). Crimson Horizon에는 인물이 없습니다.")
    ko.append("- 배경에는 창, 사이드바, 카드, 버튼, 입력창, 읽을 수 있는 텍스트가 없으며 EXIF/작성자 메타데이터도 제거되어 있습니다.\n")
    ko.append("에셋은 Patty Code 저장소의 일부로 MIT 라이선스 아래 배포되며, © Patty Code Contributors입니다.")
    ko.append("사람 검수: Patty Code Contributors (릴리스 PR 검토).\n")
    ko.append(f"생성 날짜: {today}\n")
    ko.append("| 테마 | 생성기(최종 프롬프트 등가 표현) | background.webp SHA-256 | preview.webp SHA-256 |")
    ko.append("| --- | --- | --- | --- |")
    for tid, name, prompt, bg, pv in rows:
        ko.append(f"| {name} (`{tid}`) | `{prompt}` | `{bg}` | `{pv}` |")
    ko.append("")

    with open(os.path.join(DOCS, "THEME_ASSETS.md"), "w") as f:
        f.write("\n".join(en))
    with open(os.path.join(DOCS, "THEME_ASSETS.ko-KR.md"), "w") as f:
        f.write("\n".join(ko))
    print("wrote docs/THEME_ASSETS.md + .ko-KR.md")


if __name__ == "__main__":
    main()
