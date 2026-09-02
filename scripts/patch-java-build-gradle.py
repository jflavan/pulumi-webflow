#!/usr/bin/env python3
"""
Post-process the generated Java SDK build.gradle for Maven Central publishing.

pulumi-java-gen generates a build.gradle with placeholder values that need to be
filled in for Maven Central compliance. This script patches the file with:
- Correct POM metadata (license, developer info, SCM URLs)
- gradle-nexus-publish-plugin for Maven Central Portal publishing
- Proper GPG signing configuration (3-param useInMemoryPgpKeys)

The script is idempotent: running it on an already patched file is a no-op.
It fails (exit code 1) when an expected anchor is missing from the file and
the corresponding patch has not already been applied, so that changes in the
pulumi-java-gen output are noticed instead of silently producing an
incomplete POM.
"""

import re
import sys
from pathlib import Path

# Project metadata stamped into the POM.
ARTIFACT_ID = "pulumi-webflow"
POM_NAME = "Pulumi Webflow Provider"
INCEPTION_YEAR = "2025"
PROJECT_URL = "https://github.com/jdetmar/pulumi-webflow"
SCM_CONNECTION = "scm:git:git://github.com/jdetmar/pulumi-webflow.git"
SCM_DEVELOPER_CONNECTION = "scm:git:ssh://github.com:jdetmar/pulumi-webflow.git"
LICENSE_NAME = "MIT License"
LICENSE_URL = "https://opensource.org/licenses/MIT"
DEVELOPER_ID = "jdetmar"
DEVELOPER_NAME = "Justin Detmar"
DEVELOPER_EMAIL = "jdetmar@users.noreply.github.com"

# Maven Central Portal endpoints (not legacy OSSRH).
# See: https://central.sonatype.org/publish/publish-portal-api/
STAGING_URL = "https://ossrh-staging-api.central.sonatype.com/service/local/"
SNAPSHOT_URL = "https://central.sonatype.com/repository/maven-snapshots/"

NEXUS_PUBLISH_PLUGIN = 'id("io.github.gradle-nexus.publish-plugin") version "2.0.0"'

NEXUS_BLOCK = """
if (publishRepoUsername) {
    nexusPublishing {
        repositories {
            sonatype {
                nexusUrl.set(uri(publishStagingURL))
                snapshotRepositoryUrl.set(uri(publishRepoURL))
                username = publishRepoUsername
                password = publishRepoPassword
            }
        }
    }
}
"""


class PatchError(Exception):
    """Raised when patching fails due to unexpected file format."""


def _missing(what: str, anchor: str) -> PatchError:
    return PatchError(
        f"Failed to patch {what}: expected anchor {anchor!r} not found in build.gradle "
        "and the patch is not already applied. The pulumi-java-gen output format may "
        "have changed; update scripts/patch-java-build-gradle.py."
    )


def replace_literal(content: str, old: str, new: str, what: str, *, count: int = 0) -> str:
    """
    Replace `old` with `new`.

    Raises PatchError when `old` is absent unless `new` is already present
    (i.e. the file was patched by an earlier run).
    """
    if old in content:
        return content.replace(old, new, count) if count else content.replace(old, new)
    if new in content:
        return content
    raise _missing(what, old)


def replace_regex(
    content: str,
    pattern: str,
    repl: str,
    what: str,
    done_marker: str,
    *,
    count: int = 0,
    flags: int = 0,
) -> str:
    """
    Regex variant of replace_literal.

    `done_marker` is a literal that only exists once the patch has been applied;
    it is used to recognise an already patched file.
    """
    new_content, n = re.subn(pattern, repl, content, count=count, flags=flags)
    if n:
        return new_content
    if done_marker in content:
        return content
    raise _missing(what, pattern)


def replace_in_block(content: str, block_pattern: str, pattern: str, repl: str, what: str) -> str:
    """
    Replace the first match of `pattern` inside the first match of `block_pattern`.

    Unlike replace_regex there is no file-wide done-marker: the patch is a no-op
    when the block already contains `repl`, and a PatchError is raised when the
    block is missing or contains no line matching `pattern`. This keeps the
    "already patched" check scoped to the block, so an identical value elsewhere
    in the file cannot mask a missing anchor.
    """
    block_match = re.search(block_pattern, content)
    if not block_match:
        raise _missing(what, block_pattern)
    block = block_match.group(0)
    if repl in block:
        return content
    new_block, n = re.subn(pattern, repl, block, count=1)
    if not n:
        raise _missing(what, f"{pattern} inside {block_pattern}")
    return content[: block_match.start()] + new_block + content[block_match.end() :]


def insert_after_anchor(content: str, anchor: str, addition: str, what: str, done_marker: str) -> str:
    """Insert `addition` right after `anchor` unless `done_marker` is already present."""
    if done_marker in content:
        return content
    if anchor not in content:
        raise _missing(what, anchor)
    return content.replace(anchor, anchor + addition, 1)


def insert_before_anchor(content: str, anchor: str, addition: str, what: str, done_marker: str) -> str:
    """Insert `addition` right before `anchor` unless `done_marker` is already present."""
    if done_marker in content:
        return content
    if anchor not in content:
        raise _missing(what, anchor)
    return content.replace(anchor, addition + anchor, 1)


def patch_build_gradle(filepath: str) -> None:
    """
    Patch the generated build.gradle file with Maven Central publishing configuration.

    Args:
        filepath: Path to the build.gradle file to patch

    Raises:
        FileNotFoundError: If the file doesn't exist
        PermissionError: If the file can't be read/written
        PatchError: If the file format is unexpected and patching fails
    """
    path = Path(filepath)

    if not path.exists():
        raise FileNotFoundError(f"build.gradle not found: {filepath}")

    try:
        content = path.read_text(encoding="utf-8")
    except PermissionError:
        raise PermissionError(f"Cannot read build.gradle (permission denied): {filepath}")

    original_content = content

    # --- Plugins -----------------------------------------------------------
    content = insert_after_anchor(
        content,
        'id("maven-publish")',
        f"\n    {NEXUS_PUBLISH_PLUGIN}",
        "nexus publish plugin",
        done_marker="io.github.gradle-nexus.publish-plugin",
    )

    # --- Signing / publishing variables -----------------------------------
    content = insert_before_anchor(
        content,
        'def signingKey = System.getenv("SIGNING_KEY")',
        'def signingKeyId = System.getenv("SIGNING_KEY_ID")\n',
        "signingKeyId variable",
        done_marker="def signingKeyId",
    )

    staging_default = f'def publishStagingURL = System.getenv("PUBLISH_STAGING_URL") ?: "{STAGING_URL}"'
    if "def publishStagingURL" not in content:
        content = insert_before_anchor(
            content,
            'def publishRepoUsername = System.getenv("PUBLISH_REPO_USERNAME")',
            f"{staging_default}\n",
            "publishStagingURL variable",
            done_marker="def publishStagingURL",
        )
    else:
        # publishStagingURL exists; make sure it has a default value.
        content = replace_regex(
            content,
            r'def publishStagingURL = System\.getenv\("PUBLISH_STAGING_URL"\)(?![ \t]*\?:)',
            staging_default,
            "publishStagingURL default",
            done_marker='System.getenv("PUBLISH_STAGING_URL") ?:',
            count=1,
        )

    # Update publishRepoURL default to Maven Central snapshots (only if no default exists)
    content = replace_regex(
        content,
        r'def publishRepoURL = System\.getenv\("PUBLISH_REPO_URL"\)(?![ \t]*\?:)',
        f'def publishRepoURL = System.getenv("PUBLISH_REPO_URL") ?: "{SNAPSHOT_URL}"',
        "publishRepoURL default",
        done_marker='System.getenv("PUBLISH_REPO_URL") ?:',
        count=1,
    )

    # --- Maven coordinates and POM metadata --------------------------------
    content = replace_literal(
        content, 'artifactId = "webflow"', f'artifactId = "{ARTIFACT_ID}"', "artifactId"
    )
    content = replace_literal(
        content, 'inceptionYear = ""', f'inceptionYear = "{INCEPTION_YEAR}"', "inceptionYear"
    )

    # POM name - anchored on the pom block so that it does not match other name fields.
    content = replace_regex(
        content,
        r'(pom \{[^}]*?)name = ""',
        rf'\1name = "{POM_NAME}"',
        "pom name",
        done_marker=f'name = "{POM_NAME}"',
        count=1,
    )

    # Project and SCM URLs. Older pulumi-java-gen emitted "https://example.com"
    # placeholders; newer versions fill these from the schema's repository URL, so
    # accept any quoted value and normalise it to the Maven Central form.
    content = replace_regex(
        content,
        r'(pom \{[^}]*?)url = "[^"]*"',
        rf'\1url = "{PROJECT_URL}"',
        "project url",
        done_marker=f'url = "{PROJECT_URL}"',
        count=1,
        flags=re.DOTALL,
    )
    content = replace_regex(
        content,
        r'(scm \{[^}]*?)connection = "[^"]*"',
        rf'\1connection = "{SCM_CONNECTION}"',
        "scm connection",
        done_marker=f'connection = "{SCM_CONNECTION}"',
        count=1,
    )
    content = replace_regex(
        content,
        r'(scm \{[^}]*?)developerConnection = "[^"]*"',
        rf'\1developerConnection = "{SCM_DEVELOPER_CONNECTION}"',
        "scm developerConnection",
        done_marker=f'developerConnection = "{SCM_DEVELOPER_CONNECTION}"',
        count=1,
    )
    # SCM url. This is deliberately not a replace_regex call: its done-marker
    # would have to be `url = "<PROJECT_URL>"`, which the project url patch
    # above has already inserted into the pom block, so a file-wide marker
    # would mask a missing scm url instead of raising. Scope the search to the
    # scm block itself; the url line may appear anywhere within it.
    content = replace_in_block(
        content,
        r"\bscm \{[^}]*\}",
        r'url = "[^"]*"',
        f'url = "{PROJECT_URL}"',
        "scm url",
    )

    # License block - anchored so that the pom name / developer name are not touched.
    content = replace_regex(
        content,
        r'(licenses \{[^}]*?license \{[^}]*?)name = ""',
        rf'\1name = "{LICENSE_NAME}"',
        "license name",
        done_marker=f'name = "{LICENSE_NAME}"',
    )
    content = replace_regex(
        content,
        r'(licenses \{[^}]*?license \{[^}]*?)url = ""',
        rf'\1url = "{LICENSE_URL}"',
        "license url",
        done_marker=f'url = "{LICENSE_URL}"',
    )

    # Developer block
    content = replace_regex(
        content,
        r'(developers \{[^}]*?developer \{[^}]*?)id = ""',
        rf'\1id = "{DEVELOPER_ID}"',
        "developer id",
        done_marker=f'id = "{DEVELOPER_ID}"',
    )
    content = replace_regex(
        content,
        r'(developers \{[^}]*?developer \{[^}]*?)name = ""',
        rf'\1name = "{DEVELOPER_NAME}"',
        "developer name",
        done_marker=f'name = "{DEVELOPER_NAME}"',
    )
    content = replace_regex(
        content,
        r'(developers \{[^}]*?developer \{[^}]*?)email = ""',
        rf'\1email = "{DEVELOPER_EMAIL}"',
        "developer email",
        done_marker=f'email = "{DEVELOPER_EMAIL}"',
    )

    # --- Signing: use the 3-parameter form so the key id is honoured --------
    content = replace_literal(
        content,
        "useInMemoryPgpKeys(signingKey, signingPassword)",
        "useInMemoryPgpKeys(signingKeyId, signingKey, signingPassword)",
        "useInMemoryPgpKeys",
    )

    # --- nexusPublishing block ---------------------------------------------
    if "nexusPublishing {" not in content:
        # Insert after the closing brace of the top-level publishing block.
        publishing_match = re.search(r"(publishing \{.*?^\})\s*\n", content, re.MULTILINE | re.DOTALL)
        if not publishing_match:
            raise _missing("nexusPublishing block", "publishing { ... }")
        insert_pos = publishing_match.end()
        content = content[:insert_pos] + NEXUS_BLOCK + content[insert_pos:]

    # --- Write -------------------------------------------------------------
    try:
        path.write_text(content, encoding="utf-8")
    except PermissionError:
        raise PermissionError(f"Cannot write to build.gradle (permission denied): {filepath}")

    if content == original_content:
        print(f"No changes needed for {filepath} (already patched)")
    else:
        print(f"Successfully patched {filepath}")


def main() -> int:
    """Main entry point."""
    if len(sys.argv) != 2 or sys.argv[1] in ("-h", "--help"):
        print(f"Usage: {sys.argv[0]} <build.gradle path>", file=sys.stderr)
        return 1

    try:
        patch_build_gradle(sys.argv[1])
        return 0
    except (FileNotFoundError, PermissionError, PatchError) as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
