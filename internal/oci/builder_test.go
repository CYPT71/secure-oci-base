package oci

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"io"
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
	entries := map[string]int64{}
	for {
		h, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			t.Fatal(nextErr)
		}
		entries[h.Name] = h.Mode
	}
	for name, mode := range map[string]int64{"app/service": 0555, "etc/ssl/certs/": 0755, "tmp/": 01777, "var/tmp/": 01777} {
		if entries[name] != mode {
			t.Fatalf("%s mode = %#o, want %#o", name, entries[name], mode)
		}
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

func TestNormalizeValidation(t *testing.T) {
	tests := []struct {
		name    string
		options Options
		want    string
	}{
		{"missing inputs", Options{}, "binary and output"},
		{"architecture", Options{Binary: "x", Output: t.TempDir() + "/out", Architecture: "386"}, "unsupported architecture"},
		{"operating system", Options{Binary: "x", Output: t.TempDir() + "/out", OS: "darwin"}, "unsupported operating system"},
		{"entrypoint", Options{Binary: "x", Output: t.TempDir() + "/out", Entrypoint: "app/service"}, "entrypoint"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := normalize(&tt.options); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestBuildErrors(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "binary")
	if err := os.WriteFile(file, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		options Options
		want    string
	}{
		{"missing binary", Options{Binary: filepath.Join(dir, "missing"), Output: filepath.Join(dir, "one")}, "stat binary"},
		{"directory binary", Options{Binary: dir, Output: filepath.Join(dir, "two")}, "regular file"},
		{"existing output", Options{Binary: file, Output: dir}, "output already exists"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Build(tt.options); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestLabelsFromPairs(t *testing.T) {
	labels, err := LabelsFromPairs([]string{"z=last", "a=first=value"})
	if err != nil {
		t.Fatal(err)
	}
	if labels["a"] != "first=value" || labels["z"] != "last" {
		t.Fatalf("labels = %#v", labels)
	}
	if _, err := LabelsFromPairs([]string{"bad"}); err == nil {
		t.Fatal("malformed label accepted")
	}
}

func FuzzLabelsFromPairs(f *testing.F) {
	f.Add("security.tls.minimum=1.2")
	f.Add("invalid")
	f.Fuzz(func(t *testing.T, value string) {
		_, _ = LabelsFromPairs([]string{value})
	})
}

func FuzzEntrypointValidation(f *testing.F) {
	f.Add("/app/service")
	f.Add("../../escape")
	f.Fuzz(func(t *testing.T, entrypoint string) {
		dir := t.TempDir()
		options := Options{Binary: "service", Output: filepath.Join(dir, "image"), Entrypoint: entrypoint}
		_ = normalize(&options)
	})
}

func TestWriteLayoutErrors(t *testing.T) {
	if err := writeLayout("/dev/null", []byte("{}"), nil); err == nil {
		t.Fatal("invalid root accepted")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "blobs"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeLayout(dir, []byte("{}"), nil); err == nil {
		t.Fatal("file blobs path accepted")
	}
	for _, tt := range []struct {
		name  string
		setup func(string)
	}{
		{"layout path directory", func(root string) {
			if err := os.Mkdir(filepath.Join(root, "oci-layout"), 0755); err != nil {
				t.Fatal(err)
			}
		}},
		{"index path directory", func(root string) {
			if err := os.Mkdir(filepath.Join(root, "index.json"), 0755); err != nil {
				t.Fatal(err)
			}
		}},
		{"blob path directory", func(root string) {
			d := newDescriptor("x", []byte("data"))
			if err := os.Mkdir(filepath.Join(root, "blobs", "sha256", strings.TrimPrefix(d.Digest, "sha256:")), 0755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "blobs", "sha256"), 0755); err != nil {
				t.Fatal(err)
			}
			tt.setup(root)
			if err := writeLayout(root, []byte("{}"), [][]byte{[]byte("data")}); err == nil {
				t.Fatal("write error not returned")
			}
		})
	}
}

func blobPath(root, digest string) string {
	return filepath.Join(root, "blobs", "sha256", strings.TrimPrefix(digest, "sha256:"))
}
