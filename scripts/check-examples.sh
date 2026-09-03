#!/usr/bin/env bash
# Type-check every TypeScript example against the locally built Node.js SDK.
#
# For each examples/*/typescript* directory that has a package.json this script:
#   1. packs sdk/nodejs/bin into a tarball (the same directory `npm publish` runs from,
#      so this also verifies the tarball contains the compiled JavaScript),
#   2. installs the example's dependencies with @jdetmar/pulumi-webflow replaced by
#      that tarball (without touching package.json or writing a lockfile),
#   3. runs `tsc --noEmit`.
#
# Usage: scripts/check-examples.sh            (or `make check_examples`)
# Requires: node/npm on PATH and a prior `make build_nodejs`.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SDK_DIR="${ROOT}/sdk/nodejs/bin"
NODE_MODULE_NAME="@jdetmar/pulumi-webflow"

if [[ ! -f "${SDK_DIR}/package.json" || ! -f "${SDK_DIR}/index.js" ]]; then
    echo "error: ${SDK_DIR} does not contain a built SDK (package.json + index.js)." >&2
    echo "       Run 'make build_nodejs' first." >&2
    exit 1
fi

WORK_DIR="$(mktemp -d)"
trap 'rm -rf "${WORK_DIR}"' EXIT

echo "==> Packing ${NODE_MODULE_NAME} from ${SDK_DIR}"
# `npm pack --json` prints an array describing the tarball; --pack-destination puts it in WORK_DIR.
SDK_TGZ="${WORK_DIR}/$(cd "${SDK_DIR}" && npm pack --silent --pack-destination "${WORK_DIR}")"
if [[ ! -f "${SDK_TGZ}" ]]; then
    echo "error: npm pack did not produce a tarball in ${WORK_DIR}" >&2
    exit 1
fi
# List to a file first: with `set -o pipefail`, `tar | grep -q` would fail spuriously
# because grep exits on the first match and tar receives SIGPIPE.
tar -tzf "${SDK_TGZ}" > "${WORK_DIR}/contents.txt"
if ! grep -q '^package/index\.js$' "${WORK_DIR}/contents.txt"; then
    echo "error: packed SDK tarball does not contain package/index.js; the published package would have no JavaScript." >&2
    head -n 50 "${WORK_DIR}/contents.txt" >&2
    exit 1
fi

failed=()
checked=0
shopt -s nullglob
for dir in "${ROOT}"/examples/*/typescript*/; do
    dir="${dir%/}"
    [[ -f "${dir}/package.json" ]] || continue
    checked=$((checked + 1))
    rel="${dir#"${ROOT}"/}"
    echo "==> ${rel}"
    if (
        cd "${dir}"
        # Installing the tarball explicitly overrides the registry spec for
        # ${NODE_MODULE_NAME} in package.json for this run only.
        npm install --no-audit --no-fund --no-save --no-package-lock "${SDK_TGZ}"
        npx --no-install tsc --noEmit -p .
    ); then
        echo "    ok: ${rel}"
    else
        echo "    FAILED: ${rel}" >&2
        failed+=("${rel}")
    fi
done

echo
echo "Checked ${checked} TypeScript example(s)."
if (( ${#failed[@]} > 0 )); then
    echo "Failed (${#failed[@]}):" >&2
    printf '  - %s\n' "${failed[@]}" >&2
    exit 1
fi
if (( checked == 0 )); then
    echo "error: no examples/*/typescript*/package.json found" >&2
    exit 1
fi
echo "All TypeScript examples type-check against the local SDK."
