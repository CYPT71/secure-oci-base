# secure-oci-base

[![Go quality](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-go-quality.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-go-quality.yml)
[![OCI validation](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-oci-validation.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-oci-validation.yml)
[![Pull request policy](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-pull-request.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-pull-request.yml)
[![Regression suite](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-regression-suite.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-regression-suite.yml)
[![Reproducibility](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-reproducibility.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-reproducibility.yml)
[![Runtime integration](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-runtime.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-runtime.yml)
[![Security analysis](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-security.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-security.yml)
[![CodeQL analysis](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-codeql.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-codeql.yml)
[![Integration validation](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-test.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-test.yml)
[![Fuzz validation](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-fuzz.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-fuzz.yml)
[![Release evidence](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-release.yml/badge.svg?branch=main)](https://github.com/CYPT71/secure-oci-base/actions/workflows/ci-release.yml)

`secure-oci-base` is a Go 1.25.8 command that turns one static Linux Go executable into an OCI Image Layout. It uses only the Go standard library: no Docker daemon, BuildKit, Podman, CGO, or OCI Go library is involved in layout generation.

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

### Threat model and reproducibility boundary

CI treats pull requests, artifacts, registry responses, and generated OCI layouts as untrusted. Validation independently checks layout digests, descriptor sizes, layer paths, and hostile mutations before a runtime test or release step consumes an artifact. Reproducible means that, on the fixed runner image with the pinned Go toolchain, the same revision and build inputs produce identical Linux `amd64` executable and OCI-layout bytes; registry manifests, SBOM/provenance attestations, signatures, and release metadata are deliberately external evidence and are not claimed to be byte-identical.



All CI jobs run on the fixed GitHub-hosted `ubuntu-24.04` image, use
read-only repository permissions unless publication requires package access,
and set an explicit job timeout. Actions are pinned by commit SHA and checked
by the workflow-policy verifier. The badges above show the status of the
default branch; open a badge to inspect its run logs and downloadable evidence.

| Workflow | Trigger | Validation and evidence |
| --- | --- | --- |
| **Go quality** | Push, pull request, weekly schedule | Records the Go environment; checks formatting, `go vet`, tests, native race detection, 85% coverage, and Linux `amd64`/`arm64` compilation. |
| **OCI validation** | Push and pull request | Builds a deterministic layout, verifies every descriptor/blob/layer, rejects hostile mutations, and uploads validation, checksum, and filesystem evidence. |
| **Pull-request policy** | Pull request | Rejects unsafe workflow constructs, unpinned actions, forbidden tracked build output, policy markers, and whitespace errors relative to the PR base. |
| **Regression suite** | Push and pull request | Executes all package, race, explicit CLI/OCI/mTLS, coverage, OCI structural, and hostile-layout regression tests. |
| **Reproducibility** | Push, pull request, weekly schedule | Builds the executable and OCI layout twice in isolated temporary directories, compares every byte, and uploads SHA-256 evidence. |
| **Runtime integration** | Push and pull request | Generates a static HTTP API, builds its OCI layout through the multi-stage Docker consumer, verifies `/healthz` returns `PONG` and `/` returns `HELLO WORLD` under a restricted runtime, and checks the Kubernetes restricted-runtime manifest contract offline. |
| **Security analysis** | Push, pull request, weekly schedule | Validates workflow pinning/safety, runs `go vet`, pinned `govulncheck`, and race tests, and rejects risky process-execution or insecure-TLS APIs. |
| **CodeQL analysis** | Push, pull request, weekly schedule | Uses GitHub's maintained CodeQL bundle to build and analyze Go, then uploads results to the repository security dashboard. |
| **Integration validation** | Main push and main-targeted pull request | Produces both supported Linux architecture binaries and verifies an end-to-end `amd64` layout. |
| **Fuzz validation** | Pull request and weekly schedule | Runs bounded fuzzing against label parsing and entrypoint validation boundaries. |
| **Release evidence** | Semantic version tag (`vMAJOR.MINOR.PATCH`) | Separates validated-layout build, protected-environment publication, digest signing, and GitHub release creation; publishes BuildKit SBOM/provenance evidence and attaches a Docker-loadable tarball plus reports ZIP. |

Run all checks locally:

```bash
go test ./...
go test -race ./...
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
go vet ./...
```

GitHub Actions verifies formatting, vet, normal and race tests, enforces at least 85% statement coverage, builds a static sample, and checks the OCI artifacts. Before any layout is generated, CI verifies the executable is an ELF64 binary for the requested architecture and rejects binaries with a `PT_INTERP` program header (dynamic linking). Pushing a semantic version tag (`vMAJOR.MINOR.PATCH`) creates the protected release after layout validation, publication, and digest-signature gates succeed. It attaches `secure-oci-base-image.tar`, which can be loaded with `docker load --input secure-oci-base-image.tar`, and `secure-oci-base-reports.zip`, which contains checksums, traceability, validation output, and the verified OCI layout. The workflow also publishes the image to `ghcr.io/<owner>/<repository>@sha256:<digest>` using `GITHUB_TOKEN` with `packages:write`.

### GHCR deployment

Pull the published image with an OCI-capable client or configure your runtime to use `ghcr.io/<owner>/<repository>@sha256:<digest>`. Ensure the deployment supplies the runtime hardening options described above.

### Artifact deployment

1. Open the release created for the semantic version tag and download `secure-oci-base-image.tar` and `secure-oci-base-reports.zip`.
2. Verify `image-tar.sha256`, then load the ready-to-run image: `sha256sum --check image-tar.sha256 && docker load --input secure-oci-base-image.tar`.
3. Extract the evidence bundle: `unzip secure-oci-base-reports.zip -d release-reports`.
4. Inspect `release-reports/publication-link.txt`, `signature-verification.json`, `image.digest`, and `verified-layout.tar` to independently link the validated layout to the signed published image digest.

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
