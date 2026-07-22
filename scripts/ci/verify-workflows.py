#!/usr/bin/env python3
"""Semantically validate GitHub Actions workflow security invariants."""
import json
import pathlib
import re
import subprocess
import sys

RUBY_YAML_TO_JSON = r'''
require "yaml"
require "json"
path = ARGV.fetch(0)
puts JSON.generate(YAML.safe_load(File.read(path), aliases: false))
'''
BOOTSTRAP_ACTIONS = {
    "actions/setup-go@v5",
    "docker/setup-buildx-action@v3",
    "github/codeql-action/init@v4",
    "github/codeql-action/analyze@v4",
    "github/codeql-action/autobuild@v4",
    "sigstore/cosign-installer@v3",
}


def load_workflow(path: pathlib.Path):
    result = subprocess.run(
        ["ruby", "-ryaml", "-rjson", "-e", RUBY_YAML_TO_JSON, str(path)],
        check=False, capture_output=True, text=True,
    )
    if result.returncode:
        raise ValueError(result.stderr.strip() or "YAML parser failed")
    return json.loads(result.stdout)


def walk(value):
    if isinstance(value, dict):
        yield value
        for child in value.values():
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


errors = []
for workflow in sorted(pathlib.Path(".github/workflows").glob("*.y*ml")):
    try:
        data = load_workflow(workflow)
    except (ValueError, json.JSONDecodeError) as exc:
        errors.append(f"{workflow}: invalid YAML: {exc}")
        continue
    if not isinstance(data, dict):
        errors.append(f"{workflow}: root must be a mapping")
        continue
    triggers = data.get("on", data.get(True))
    trigger_names = set(triggers) if isinstance(triggers, dict) else set(triggers or [])
    if {"pull_request_target", "workflow_run"} & trigger_names:
        errors.append(f"{workflow}: privileged untrusted trigger")
    jobs = data.get("jobs")
    if not isinstance(jobs, dict) or not jobs:
        errors.append(f"{workflow}: no jobs defined")
        continue
    for job_name, job in jobs.items():
        if not isinstance(job, dict):
            errors.append(f"{workflow}: job {job_name} must be a mapping")
            continue
        if job.get("runs-on") != "ubuntu-24.04":
            errors.append(f"{workflow}: job {job_name} must use fixed ubuntu-24.04 runner")
        if not isinstance(job.get("timeout-minutes"), int):
            errors.append(f"{workflow}: job {job_name} must declare integer timeout")
        for step in job.get("steps", []):
            if not isinstance(step, dict):
                errors.append(f"{workflow}: job {job_name} contains invalid step")
                continue
            action = step.get("uses")
            if isinstance(action, str):
                if action.startswith(("./", "docker://")):
                    errors.append(f"{workflow}: disallowed action source {action}")
                elif action not in BOOTSTRAP_ACTIONS and not re.fullmatch(r"[^@\s]+@[0-9a-f]{40}", action):
                    errors.append(f"{workflow}: action is not SHA pinned: {action}")
            command = step.get("run")
            if isinstance(command, str) and "set -euo pipefail" not in command:
                errors.append(f"{workflow}: job {job_name} shell step lacks strict mode")
        for node in walk(job):
            if isinstance(node, str) and re.search(r"\$\{\{\s*github\.event\.pull_request\.(title|body|head\.ref)", node):
                errors.append(f"{workflow}: untrusted PR value interpolated into shell")
if errors:
    print("WORKFLOW_VALIDATION_FAILURE", *errors, sep="\n", file=sys.stderr)
    sys.exit(1)
print("WORKFLOW_VALIDATION_OK")
