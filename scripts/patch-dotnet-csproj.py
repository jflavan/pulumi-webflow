#!/usr/bin/env python3
# Copyright 2025, Justin Detmar.
# SPDX-License-Identifier: MIT
#
# This is an unofficial, community-maintained Pulumi provider for Webflow.
# Not affiliated with, endorsed by, or supported by Pulumi Corporation or Webflow, Inc.
"""Post-process the generated .NET SDK project file.

Pulumi's .NET code generator hardcodes ``<TargetFramework>net6.0</TargetFramework>`` and offers
no schema setting to change it. This script rewrites that line so the package is compiled for
every framework in TARGET_FRAMEWORKS. It runs as part of ``make codegen`` (see the ``sdk/dotnet``
rule in the Makefile), so the committed project file always matches regenerated output.

The script is idempotent and fails loudly when the expected anchor is missing, so a change in the
generator's template cannot silently ship a package with the wrong target frameworks.
"""

import re
import sys
from pathlib import Path

TARGET_FRAMEWORKS = "net8.0;net10.0"

GENERATED_PATTERN = re.compile(r"^(\s*)<TargetFramework>[^<]+</TargetFramework>\s*$", re.MULTILINE)
PATCHED_PATTERN = re.compile(r"^(\s*)<TargetFrameworks>([^<]+)</TargetFrameworks>\s*$", re.MULTILINE)


class PatchError(Exception):
    """Raised when the project file does not look like generator output."""


def patch(content: str) -> tuple[str, bool]:
    """Return the patched content and whether anything changed."""
    already = PATCHED_PATTERN.search(content)
    if already:
        if already.group(2) == TARGET_FRAMEWORKS:
            return content, False
        return PATCHED_PATTERN.sub(rf"\1<TargetFrameworks>{TARGET_FRAMEWORKS}</TargetFrameworks>", content, count=1), True

    if len(GENERATED_PATTERN.findall(content)) != 1:
        raise PatchError(
            "expected exactly one <TargetFramework> element in the generated project file; "
            "the Pulumi .NET generator template may have changed"
        )
    return GENERATED_PATTERN.sub(rf"\1<TargetFrameworks>{TARGET_FRAMEWORKS}</TargetFrameworks>", content, count=1), True


def main(argv: list[str]) -> int:
    if len(argv) != 2:
        print(f"usage: {argv[0]} <path/to/Project.csproj>", file=sys.stderr)
        return 2
    path = Path(argv[1])
    if not path.is_file():
        print(f"error: {path} does not exist", file=sys.stderr)
        return 1
    try:
        patched, changed = patch(path.read_text(encoding="utf-8"))
    except PatchError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1
    if changed:
        path.write_text(patched, encoding="utf-8")
        print(f"patched {path}: TargetFrameworks={TARGET_FRAMEWORKS}")
    else:
        print(f"{path}: already patched (TargetFrameworks={TARGET_FRAMEWORKS})")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
