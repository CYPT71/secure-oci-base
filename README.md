# secure-oci-base

`secure-oci-base` creates a standards-compliant [OCI Image Layout](https://github.com/opencontainers/image-spec/blob/main/image-layout.md) directly from a compiled executable. It does **not** require Docker, BuildKit, Podman, a container daemon, CGO, or third-party Go modules.

## Security and reproducibility defaults

* The generated image runs as the non-root numeric user `65532:65532`.
* The application is the only layer payload, at `/app/service`, with mode `0555`.
* Output is written to a fresh temporary directory and atomically renamed; an existing output path is refused.
* Tar and gzip timestamps are fixed at the Unix epoch. The default image creation time is also the Unix epoch, enabling byte-for-byte repeatable output for identical inputs.
* All OCI blobs use `sha256`; the configuration records the uncompressed layer digest (`diff_id`).
* The builder rejects non-regular or non-executable input files and unsafe entrypoint paths.

The supplied executable should be a static Linux Go binary. Build it without CGO, for example:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o service ./cmd/your-service
go run ./cmd/oci-builder -binary ./service -output ./oci-image -arch amd64 \
  -label security.tls.minimum=1.2 -label security.mtls=required
```

The resulting directory contains `oci-layout`, `index.json`, and content-addressed blobs under `blobs/sha256`, and can be consumed by OCI-compatible tooling. The index carries the requested image name and tag as `org.opencontainers.image.ref.name`.

## CLI

```text
oci-builder -binary PATH -output DIRECTORY [options]

  -arch ARCH             OCI architecture (defaults to the Go host architecture)
  -os OS                 OCI operating system (default linux)
  -entrypoint PATH       absolute in-image command (default /app/service)
  -image NAME            image name annotation (default secure-oci-base)
  -tag TAG               image tag annotation (default latest)
  -created RFC3339       creation time (default 1970-01-01T00:00:00Z)
  -label KEY=VALUE       image label; may be specified repeatedly
```

Labels are metadata only: TLS/mTLS enforcement, cgroups, namespaces, seccomp, and network policy remain runtime and orchestrator responsibilities.

## Development

The project targets Go 1.22 or newer and uses only the standard library.

```bash
go test ./...
go test -race ./...
go vet ./...
```
