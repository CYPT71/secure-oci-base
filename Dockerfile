# syntax=docker/dockerfile:1
# Build context must contain an OCI layout at ./oci-image (for example, a
# downloaded secure-oci-layout Actions artifact).
FROM alpine:3.20 AS verify
WORKDIR /layout
COPY oci-image/ ./
# Validate the required OCI layout files and every content-addressed blob.
RUN test -f oci-layout && test -f index.json && \
    test "$(find blobs/sha256 -type f | wc -l)" -eq 3 && \
    for blob in blobs/sha256/*; do test "$(sha256sum "$blob" | cut -d' ' -f1)" = "$(basename "$blob")"; done

FROM alpine:3.20 AS unpack
RUN apk add --no-cache tar gzip
COPY --from=verify /layout/blobs/sha256 /blobs
COPY --from=verify /layout/index.json /index.json
# Extract the sole compressed layer selected by the OCI manifest. The layout
# created by this project has one manifest and one layer.
RUN manifest=$(sed -n 's/.*"digest":"sha256:\([a-f0-9]*\)".*/\1/p' /index.json | head -1) && \
    test -n "$manifest" && \
    layer=$(sed -n 's/.*"digest":"sha256:\([a-f0-9]*\)".*/\1/p' "/blobs/$manifest" | tail -1) && \
    mkdir /rootfs && gzip -dc "/blobs/$layer" | tar -x -C /rootfs

FROM scratch
COPY --from=unpack /rootfs/ /
USER 65532:65532
ENTRYPOINT ["/app/service"]
