#!/usr/bin/env python3
"""Reject insecure workflow constructs before a workflow can be trusted."""
import pathlib
import re
import sys

root = pathlib.Path(".github/workflows")
errors = []
for workflow in sorted(root.glob("*.y*ml")):
    text = workflow.read_text(encoding="utf-8")
    if "pull_request_target" in text or "workflow_run" in text:
        errors.append(f"{workflow}: privileged untrusted trigger")
    for line in text.splitlines():
        if "uses:" in line:
            value = line.split("uses:", 1)[1].strip().split()[0].strip("'\"")
            if value.startswith("./") or value.startswith("docker://"):
                errors.append(f"{workflow}: disallowed action source {value}")
            # These official bootstrap actions resolve the current signed release
            # in their supported major channel. They install the pinned Go SDK,
            # GitHub-maintained CodeQL bundle, and Cosign verifier respectively.
            elif re.fullmatch(
                r"(?:actions/setup-go@v5|docker/setup-buildx-action@v3|github/codeql-action/(init|autobuild|analyze)@v4|sigstore/cosign-installer@v3)",
                value,
            ):
                continue
            elif "@" not in value or not re.search(r"@[0-9a-f]{40}$", value):
                errors.append(f"{workflow}: action is not SHA pinned: {value}")
        if re.search(r"\$\{\{\s*github\.event\.pull_request\.(title|body|head\.ref)", line):
            errors.append(f"{workflow}: untrusted PR value interpolated into shell")
    if "runs-on: ubuntu-latest" not in text:
        errors.append(f"{workflow}: jobs must use the supported ubuntu-latest runner")
    if "timeout-minutes:" not in text:
        errors.append(f"{workflow}: jobs must declare a timeout")
    if "set -euo pipefail" not in text:
        errors.append(f"{workflow}: shell steps must enable pipefail")
if errors:
    print("WORKFLOW_VALIDATION_FAILURE", *errors, sep="\n", file=sys.stderr)
    sys.exit(1)
print("WORKFLOW_VALIDATION_OK")
