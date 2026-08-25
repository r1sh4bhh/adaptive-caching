#!/usr/bin/env bash
#
# lint-arch.sh — enforce the layering rule from context.md §3.6.
#
# The cache core must never depend on the observation or experimentation
# layers. If it does, the UI stops being deletable and every later phase pays
# for it. This check is what keeps the UI cheap for the whole project.
#
# Exits non-zero (and names the offending import path) on violation.

set -euo pipefail

MODULE="github.com/r1sh4bhh/adaptive-caching"

# Packages whose dependency closure is checked.
GUARDED_PACKAGES=(
  "${MODULE}/cache/..."
)

# Package prefixes the guarded packages may never depend on.
FORBIDDEN=(
  "${MODULE}/server"
  "${MODULE}/tui"
  "${MODULE}/benchmark"
  "${MODULE}/adaptive"
  "${MODULE}/ui"
)

cd "$(dirname "$0")/.."

status=0

for pkg_pattern in "${GUARDED_PACKAGES[@]}"; do
  # go list -deps prints the full transitive dependency closure, one import
  # path per line, so an indirect violation is caught as well as a direct one.
  for pkg in $(go list "${pkg_pattern}" 2>/dev/null); do
    deps=$(go list -deps "${pkg}" 2>/dev/null || true)
    for forbidden in "${FORBIDDEN[@]}"; do
      while IFS= read -r dep; do
        [ -z "${dep}" ] && continue
        if [ "${dep}" = "${forbidden}" ] || [[ "${dep}" == "${forbidden}/"* ]]; then
          echo "ARCH VIOLATION: ${pkg} imports ${dep}" >&2
          status=1
        fi
      done <<< "${deps}"
    done
  done
done

if [ "${status}" -ne 0 ]; then
  echo "" >&2
  echo "cache/ must not depend on server/, tui/, benchmark/, adaptive/ or ui/." >&2
  echo "See context.md §3.6 — the layering rule is non-negotiable." >&2
  exit 1
fi

echo "lint-arch: OK — no forbidden imports in cache/"
