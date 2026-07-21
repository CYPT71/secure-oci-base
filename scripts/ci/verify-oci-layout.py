#!/usr/bin/env python3
"""Strict, dependency-free verifier for an OCI image layout produced in CI."""
import hashlib
import json
import pathlib
import sys


def fail(message):
    print(f"OCI_VALIDATION_FAILURE: {message}", file=sys.stderr)
    raise SystemExit(1)


def blob(root, descriptor, expected_media=None):
    digest = descriptor.get("digest", "")
    if not isinstance(digest, str) or not digest.startswith("sha256:") or len(digest) != 71:
        fail(f"invalid descriptor digest: {digest!r}")
    if expected_media and descriptor.get("mediaType") != expected_media:
        fail(f"unexpected media type: {descriptor.get('mediaType')!r}")
    path = root / "blobs" / "sha256" / digest[7:]
    if not path.is_file():
        fail(f"missing blob: {digest}")
    data = path.read_bytes()
    if hashlib.sha256(data).hexdigest() != digest[7:]:
        fail(f"digest mismatch: {digest}")
    if descriptor.get("size") != len(data):
        fail(f"size mismatch: {digest}")
    return data


def main(root_name):
    root = pathlib.Path(root_name)
    if not root.is_dir():
        fail("layout root is not a directory")
    if (root / "oci-layout").read_text(encoding="utf-8") != '{"imageLayoutVersion":"1.0.0"}\n':
        fail("invalid oci-layout marker")
    try:
        index = json.loads((root / "index.json").read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        fail(f"invalid index: {error}")
    manifests = index.get("manifests")
    if index.get("schemaVersion") != 2 or not isinstance(manifests, list) or len(manifests) != 1:
        fail("index must contain exactly one schema-2 manifest")
    manifest_data = blob(root, manifests[0], "application/vnd.oci.image.manifest.v1+json")
    try:
        manifest = json.loads(manifest_data)
    except json.JSONDecodeError as error:
        fail(f"invalid manifest JSON: {error}")
    if manifest.get("schemaVersion") != 2 or not isinstance(manifest.get("layers"), list) or len(manifest["layers"]) != 1:
        fail("manifest must contain exactly one layer")
    config = blob(root, manifest.get("config", {}), "application/vnd.oci.image.config.v1+json")
    blob(root, manifest["layers"][0], "application/vnd.oci.image.layer.v1.tar+gzip")
    try:
        config_doc = json.loads(config)
    except json.JSONDecodeError as error:
        fail(f"invalid config JSON: {error}")
    if config_doc.get("architecture") not in ("amd64", "arm64") or config_doc.get("os") != "linux":
        fail("unsupported platform in config")
    print("OCI_VALIDATION_OK")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        fail("usage: verify-oci-layout.py LAYOUT")
    main(sys.argv[1])
