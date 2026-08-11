#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

make -C "$repo_root" build >/dev/null

if [ "$(go env GOOS)" != darwin ]; then
	exit 0
fi

binary="$repo_root/bin/patcode"
signature="$(/usr/bin/codesign -dv --verbose=4 "$binary" 2>&1)"
if grep -q 'linker-signed' <<<"$signature"; then
	echo "local macOS CLI retained Go's linker-signed signature" >&2
	exit 1
fi

/usr/bin/codesign --verify --strict --verbose=2 "$binary"
"$binary" --help >/dev/null
