#!/usr/bin/env bash
# Build every C# example against the local .NET SDK sources.
#
# Each examples/*/csharp project references ../../../sdk/dotnet/Community.Pulumi.Webflow.csproj
# when that file exists (and the published NuGet package otherwise), so building the examples
# here compiles the SDK for net8.0 and net10.0 and then compiles each example against it.
#
# Usage: scripts/check-dotnet-examples.sh      (or `make check_dotnet_examples`)
# Requires: the .NET 10 SDK on PATH (see .config/mise.toml).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SDK_PROJECT="${ROOT}/sdk/dotnet/Community.Pulumi.Webflow.csproj"

if ! command -v dotnet >/dev/null 2>&1; then
    echo "error: dotnet is not on PATH; install the .NET SDK pinned in .config/mise.toml" >&2
    exit 1
fi
if [[ ! -f "${SDK_PROJECT}" ]]; then
    echo "error: ${SDK_PROJECT} not found; run 'make codegen' first" >&2
    exit 1
fi
# The generated project embeds version.txt; codegen writes it during dotnet_sdk, so create a
# placeholder for a bare checkout.
if [[ ! -f "${ROOT}/sdk/dotnet/version.txt" ]]; then
    echo "0.0.0-dev" > "${ROOT}/sdk/dotnet/version.txt"
fi

export DOTNET_CLI_TELEMETRY_OPTOUT=1 DOTNET_NOLOGO=1

failed=()
checked=0
for project in "${ROOT}"/examples/*/csharp/*.csproj; do
    dir="$(dirname "${project}")"
    rel="${dir#"${ROOT}"/}"
    echo "==> ${rel}"
    if (cd "${dir}" && dotnet build --nologo -v quiet); then
        echo "    ok: ${rel}"
    else
        echo "    FAILED: ${rel}"
        failed+=("${rel}")
    fi
    checked=$((checked + 1))
done

echo
echo "Checked ${checked} C# example(s)."
if (( ${#failed[@]} > 0 )); then
    echo "Failed (${#failed[@]}):"
    printf '  - %s\n' "${failed[@]}"
    exit 1
fi
echo "All C# examples build against the local SDK (net8.0 and net10.0)."
