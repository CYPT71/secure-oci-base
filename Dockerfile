# syntax=docker/dockerfile:1@sha256:87999aa3d42bdc6bea60565083ee17e86d1f3339802f543c0d03998580f9cb89
# Build context must contain an OCI layout at ./oci-image (for example, a
# downloaded secure-oci-layout Actions artifact).
FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS verify
WORKDIR /layout
COPY oci-image/ ./
# Validate the required OCI layout files and every content-addressed blob.
RUN test -f oci-layout && test -f index.json && \
    test "$(find blobs/sha256 -type f | wc -l)" -eq 3 && \
    for blob in blobs/sha256/*; do test "$(sha256sum "$blob" | cut -d' ' -f1)" = "$(basename "$blob")"; done

FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc AS unpack
RUN apk add --no-cache jq tar gzip
COPY --from=verify /layout/blobs/sha256 /blobs
COPY --from=verify /layout/index.json /index.json
# Extract the sole compressed layer selected by the OCI manifest. The layout
# created by this project has one manifest and one layer.
RUN manifest=$(jq -er '.manifests | if length == 1 then .[0].digest else error("expected one manifest") end | select(startswith("sha256:")) | ltrimstr("sha256:")' /index.json) && \
    test -f "/blobs/$manifest" && \
    layer=$(jq -er '.layers | if length == 1 then .[0].digest else error("expected one layer") end | select(startswith("sha256:")) | ltrimstr("sha256:")' "/blobs/$manifest") && \
    test -f "/blobs/$layer" && \
    mkdir /rootfs && gzip -dc "/blobs/$layer" | tar -x --no-same-owner --no-same-permissions -C /rootfs

FROM scratch
COPY --from=unpack /rootfs/ /
USER 65532:65532
ENTRYPOINT ["/app/service"]
