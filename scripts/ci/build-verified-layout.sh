#!/usr/bin/env bash
# Build a deterministic builder binary and verify the OCI layout it creates.
set -euo pipefail

if [ "$#" -ne 3 ]; then
  echo "usage: build-verified-layout.sh OUTPUT BINARY ARCH" >&2
  exit 2
fi

output=$1
binary=$2
arch=$3

CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build -trimpath -buildvcs=true -o "$binary" ./cmd/oci-builder
scripts/ci/verify-elf.sh "$binary" "$arch"
case "$binary" in
  /*) builder="$binary" ;;
  *) builder="./$binary" ;;
esac
"$builder" -binary "$binary" -output "$output" -arch "$arch" -created 1970-01-01T00:00:00Z
python3 scripts/ci/verify-oci-layout.py "$output"
