#!/usr/bin/env bash
# Verify that a stable orchestration produced every public release channel.
set -euo pipefail

repository="${RELEASE_REPOSITORY:?RELEASE_REPOSITORY is required}"
version="${RELEASE_VERSION:?RELEASE_VERSION is required}"
cli_tag="${CLI_TAG:?CLI_TAG is required}"
desktop_tag="${DESKTOP_TAG:?DESKTOP_TAG is required}"
attempts="${VERIFY_ATTEMPTS:-6}"
delay="${VERIFY_DELAY_SECONDS:-10}"

if [[ ! "$version" =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
	echo "::error::RELEASE_VERSION must be stable semver, got: $version" >&2
	exit 1
fi
if [ "$cli_tag" != "v$version" ] || [ "$desktop_tag" != "desktop-v$version" ]; then
	echo "::error::release tags do not match version $version: cli=$cli_tag desktop=$desktop_tag" >&2
	exit 1
fi

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/patty-code-release-postflight.XXXXXX")"
cleanup() {
	case "$tmp_dir" in
	*/patty-code-release-postflight.*) rm -rf -- "$tmp_dir" ;;
	*) echo "refusing to clean unexpected postflight directory: $tmp_dir" >&2 ;;
	esac
}
trap cleanup EXIT

gh release view "$cli_tag" --repo "$repository" --json isDraft,isPrerelease,assets >"$tmp_dir/cli.json"
jq -e '
  .isDraft == false and .isPrerelease == false and
  ([.assets[].name] as $names |
    ["SHA256SUMS", "patty-code-darwin-amd64.tar.gz", "patty-code-darwin-arm64.tar.gz",
     "patty-code-linux-amd64.tar.gz", "patty-code-linux-arm64.tar.gz",
     "patty-code-windows-amd64.zip", "patty-code-windows-arm64.zip"] |
    all(. as $required | $names | index($required)))
' "$tmp_dir/cli.json" >/dev/null

gh release view "$desktop_tag" --repo "$repository" --json isDraft,isPrerelease,assets >"$tmp_dir/desktop.json"
jq -e '
  .isDraft == false and .isPrerelease == false and
  ([.assets[].name] as $names |
    ($names | index("latest.json")) and
    (["Patty Code-darwin-universal.dmg", "Patty Code-linux-amd64.deb",
      "Patty Code-linux-amd64.tar.gz", "Patty Code-windows-amd64-installer.exe",
      "Patty Code-windows-arm64-installer.exe"] |
     all(. as $required | ($names | index($required)) and ($names | index($required + ".minisig")))))
' "$tmp_dir/desktop.json" >/dev/null

for attempt in $(seq 1 "$attempts"); do
	latest="$(npm view patty-code dist-tags.latest 2>/dev/null || true)"
	if [ "$latest" = "$version" ]; then
		echo "stable release postflight OK: cli=$cli_tag desktop=$desktop_tag npm-latest=$latest"
		exit 0
	fi
	echo "npm latest -> ${latest:-<unreadable>}, want $version (attempt $attempt/$attempts)"
	if [ "$attempt" -lt "$attempts" ]; then
		sleep "$delay"
	fi
done

echo "::error::npm latest did not reach $version" >&2
exit 1
