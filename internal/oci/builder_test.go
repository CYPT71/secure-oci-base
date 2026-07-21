package oci

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildWritesValidLayout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	binary := filepath.Join(dir, "service")
	if err := os.WriteFile(binary, []byte("binary-data"), 0755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "image")
	digest, err := Build(Options{Binary: binary, Output: output, Architecture: "amd64", ImageName: "example/service", Tag: "v1", Created: time.Unix(0, 0), Labels: map[string]string{"security.tls.minimum": "1.2"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(digest, "sha256:") {
		t.Fatalf("digest = %q", digest)
	}
	if got, err := os.ReadFile(filepath.Join(output, "oci-layout")); err != nil || string(got) != "{\"imageLayoutVersion\":\"1.0.0\"}\n" {
		t.Fatalf("oci-layout = %q, %v", got, err)
	}
	var idx index
	data, err := os.ReadFile(filepath.Join(output, "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatal(err)
	}
	if len(idx.Manifests) != 1 || idx.Manifests[0].Digest != digest {
		t.Fatalf("unexpected index: %+v", idx)
	}
	if got := idx.Manifests[0].Annotations["org.opencontainers.image.ref.name"]; got != "example/service:v1" {
		t.Fatalf("reference = %q", got)
	}
	manifestData, err := os.ReadFile(blobPath(output, digest))
	if err != nil {
		t.Fatal(err)
	}
	var m manifest
	if err := json.Unmarshal(manifestData, &m); err != nil {
		t.Fatal(err)
	}
	configData, err := os.ReadFile(blobPath(output, m.Config.Digest))
	if err != nil {
		t.Fatal(err)
	}
	var config imageConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatal(err)
	}
	if config.Config.User != "65532:65532" || config.Config.Entrypoint[0] != "/app/service" || config.RootFS.DiffIDs[0] == "" {
		t.Fatalf("unsafe config: %+v", config)
	}
	layer, err := os.Open(blobPath(output, m.Layers[0].Digest))
	if err != nil {
		t.Fatal(err)
	}
	defer layer.Close()
	gz, err := gzip.NewReader(layer)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	_, err = tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	h, err := tr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != "app/service" || h.Mode != 0555 {
		t.Fatalf("unsafe layer header: %+v", h)
	}
}

func TestBuildRejectsUnsafeInputs(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "binary")
	if err := os.WriteFile(binary, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(Options{Binary: binary, Output: filepath.Join(dir, "out")}); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("err = %v", err)
	}
	if _, err := LabelsFromPairs([]string{"a=1", "a=2"}); err == nil {
		t.Fatal("duplicate labels accepted")
	}
}

func blobPath(root, digest string) string {
	return filepath.Join(root, "blobs", "sha256", strings.TrimPrefix(digest, "sha256:"))
}
