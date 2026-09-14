#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$repo_root"

fail() {
    echo "repository check failed: $*" >&2
    exit 1
}

[ -L CLAUDE.md ] || fail "CLAUDE.md must be a symlink"
[ "$(readlink CLAUDE.md)" = "AGENTS.md" ] || fail "CLAUDE.md must point to AGENTS.md"

tracked_forbidden=$(git ls-files | awk '
    /(^|\/)\.env($|\.)/ && $0 !~ /\.env\.example$/ { print }
    /(^|\/)signing\.properties$/ { print }
    /\.(jks|keystore|p12|mobileprovision)$/ { print }
    /(^|\/)ExportOptions\.plist$/ { print }
    /(^|\/)local\.properties$/ { print }
    /^ferry-data\// || /(^|\/)artifacts\// { print }
')
[ -z "$tracked_forbidden" ] || fail "tracked local data or signing material:\n$tracked_forbidden"

# Compiled output is never a source file. The largest legitimate asset is a
# 1024px app icon at about 2 MiB, so anything past 3 MiB is a build artifact
# that slipped past .gitignore and would stay in history once pushed.
oversized=$(git ls-files -z \
    | xargs -0 -n1 -I{} sh -c 'set -- $(wc -c < "{}"); [ "$1" -gt 3145728 ] && echo "{} ($1 bytes)"' \
    | sort)
[ -z "$oversized" ] || fail "tracked file is larger than 3 MiB:\n$oversized"

grep -Fq 'Android App' README.md || fail "README must describe Android"
grep -Fq 'Superseded on 2026-08-30' docs/four-digit-pairing.md || fail "pairing document must stay historical"
grep -Fq 'Apache License 2.0' README.md || fail "README license statement is missing"

# The deployment bundle ships both files; a user who copies .env.example must
# get the same image that compose.yaml would choose without it.
compose_image=$(sed -n 's/^    image: \${FERRY_IMAGE:-\(.*\)}$/\1/p' compose.yaml)
[ -n "$compose_image" ] || fail "compose.yaml must default FERRY_IMAGE to a published image"
grep -Fxq "FERRY_IMAGE=$compose_image" .env.example || fail ".env.example must name $compose_image like compose.yaml"

unpinned_actions=$(awk '
    /^[[:space:]]*- uses:/ {
        count = split($0, parts, "@")
        ref = parts[count]
        sub(/[[:space:]#].*$/, "", ref)
        if (length(ref) != 40 || ref ~ /[^0-9a-f]/) print $0
    }
' .github/workflows/*.yml)
[ -z "$unpinned_actions" ] || fail "workflow action is not pinned to a commit SHA:\n$unpinned_actions"

grep -Fxq 'trap cleanup EXIT' scripts/ferry-data.sh || fail "data cleanup must own EXIT"
grep -Fxq "trap 'exit 129' HUP" scripts/ferry-data.sh || fail "HUP must remain non-zero"
grep -Fxq "trap 'exit 130' INT" scripts/ferry-data.sh || fail "INT must remain non-zero"
grep -Fxq "trap 'exit 143' TERM" scripts/ferry-data.sh || fail "TERM must remain non-zero"

for script in scripts/*.sh; do
    sh -n "$script"
done

echo "repository policy checks passed"
