# secure-oci-base

`secure-oci-base` is a Go 1.22+ command that turns one static Linux Go executable into an OCI Image Layout. It uses only the Go standard library: no Docker daemon, BuildKit, Podman, CGO, or OCI Go library is involved in layout generation.

## Architecture and OCI layout

The CLI validates an executable, creates a deterministic `tar` layer and gzip stream, then writes the OCI image configuration, manifest, index, and content-addressed blobs. A generated layout has this shape:

```text
oci-image/
  oci-layout                 # imageLayoutVersion 1.0.0
  index.json                 # platform descriptor and image reference annotation
  blobs/sha256/
    <config digest>
    <manifest digest>
    <compressed layer digest>
```

The layer contains `/app/service`, `/etc/ssl/certs/`, `/tmp/`, and `/var/tmp/`. The last two directories are sticky writable directories; all other included directories are `0755` and the service is `0555`. The config specifies `linux`, `amd64` or `arm64`, a single compressed layer, its uncompressed `diff_id`, and `User` `65532:65532`.

## Security model

Inputs must be a regular executable and the output must not already exist. Entrypoints must be clean absolute container paths; label keys cannot be duplicated. Files are written to a fresh temporary directory and atomically renamed to avoid partial output. SHA-256 identifies every blob. Tar and gzip timestamps, gzip OS, and the default `created` value are fixed, so identical input bytes and options produce identical layouts.

The builder never reads or copies credentials, source trees, environment variables, or host CA files. TLS/mTLS labels are metadata only. **No OCI image configuration can enforce `no_new_privileges` or a read-only root filesystem**: enforce those at runtime, for example `docker run --read-only --security-opt no-new-privileges ...`, Kubernetes `securityContext.readOnlyRootFilesystem: true` and `allowPrivilegeEscalation: false`, or equivalent containerd policy.

## Build and use

Build the application statically, then create a layout:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o service ./cmd/your-service
go run ./cmd/oci-builder -binary ./service -output ./oci-image -arch amd64 \
  -label security.tls.minimum=1.2 -label security.mtls=required
```

Supported architectures are `amd64` and `arm64`; the operating system is `linux`. The CLI exits with `2` for malformed arguments and `1` for build failures. Run `go run ./cmd/oci-builder -h` for the complete flag reference.

## Reusable mTLS configuration

`internal/mtls` offers `ClientConfig` and `ServerConfig`. Both require TLS 1.2 or newer without setting a maximum version, so Go automatically enables TLS 1.3. `Options` accepts PEM CA data and `tls.Certificate` identities. A server configured with `MutualTLS: true` requires a CA bundle and verifies every client certificate.

## Testing and CI/CD


Run all checks locally:

```bash
go test ./...
go test -race ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
go vet ./...
```

GitHub Actions verifies formatting, vet, normal and race tests, enforces at least 99% statement coverage, builds a static sample, and checks the OCI artifacts. On pushes to `main`, it attempts to publish the layout to `ghcr.io/<owner>/<repository>:latest` using `GITHUB_TOKEN` and `packages:write`. If publishing is unavailable or fails, it uploads the `secure-oci-layout` artifact and reports the publishing outcome.

### GHCR deployment

Pull the published image with an OCI-capable client or configure your runtime to use `ghcr.io/<owner>/<repository>:latest`. Ensure the deployment supplies the runtime hardening options described above.

### Artifact deployment

1. Open the workflow run and download `secure-oci-layout`.
2. Extract it: `unzip secure-oci-layout.zip -d oci-image`.
3. Inspect `oci-image/index.json`, then verify blobs: `for f in oci-image/blobs/sha256/*; do test "$(sha256sum "$f" | cut -d' ' -f1)" = "$(basename "$f")"; done`.
4. Import or copy the layout using an OCI-aware runtime/tool appropriate for your environment.

## Dockerfile consumer

`Dockerfile` demonstrates a separate, multi-stage consumer. Its first stage validates `oci-layout`, `index.json`, and blob filenames; its second stage extracts the layer; the final `scratch` stage copies only the extracted root filesystem and declares the numeric non-root user and entrypoint. Put a downloaded/extracted layout at `./oci-image` and run `docker build -t service:local .`. The Dockerfile does not replace runtime read-only-rootfs or no-new-privileges settings.

## Debugging minimal containers

Use OCI tooling to inspect descriptors and verify digests before deployment. For a running minimal/distroless Docker container, `nsenter` is often preferable to `docker exec` because it does not require a shell in the image:

```bash
PID=$(docker inspect -f '{{.State.Pid}}' <container>)
sudo nsenter --target "$PID" --mount --uts --ipc --net --pid
```

Within the target namespaces, inspect `/proc/1`, `/proc/1/mountinfo`, `/proc/1/environ` (subject to permission), `ip addr`, `ip route`, and the mounted root filesystem. For containerd, first obtain the task PID with `ctr -n <namespace> tasks ls`, then use the same `nsenter` command. On Kubernetes, use an authorized node-debug session (for example `kubectl debug node/<node> -it --image=busybox`) and enter the target container PID from the node runtime.

`nsenter` requires host root/CAP_SYS_ADMIN-equivalent privileges and exposes namespaces, processes, mounts, network state, and potentially secrets. Restrict it to incident responders, audited hosts, and approved maintenance windows. Prefer `docker exec` or `kubectl exec` for normal application-level diagnostics; choose `nsenter` for namespace, mount, network, or distroless-image investigation.

## Troubleshooting and limitations

* **“binary is not executable”**: `chmod 0755 service`, then rebuild with `CGO_ENABLED=0`.
* **Unsupported architecture**: pass `-arch amd64` or `-arch arm64` and build the binary for the same target.
* **Output already exists**: select a new output directory; it is intentionally never overwritten.
* **Registry publication fails**: use the workflow artifact; confirm repository Actions permissions allow `packages:write` before retrying.
* This is a single-layer image generator, not a Dockerfile interpreter, registry client, image signer, SBOM generator, or runtime security policy engine. It does not prove that an executable is static; build it using the static command above.
