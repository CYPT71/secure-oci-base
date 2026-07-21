// Package oci writes small, deterministic OCI image layouts without a daemon.
package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	manifestMediaType = "application/vnd.oci.image.manifest.v1+json"
	configMediaType   = "application/vnd.oci.image.config.v1+json"
	layerMediaType    = "application/vnd.oci.image.layer.v1.tar+gzip"
)

// Options describes the image to create. Binary must name a regular executable
// file. Output must not already exist; this prevents accidentally replacing an
// image layout with attacker-controlled contents.
type Options struct {
	Binary       string
	Output       string
	Architecture string
	OS           string
	Entrypoint   string
	ImageName    string
	Tag          string
	Created      time.Time
	Labels       map[string]string
}

type descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Platform    *platform         `json:"platform,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}
type platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
}
type rootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}
type imageConfig struct {
	Created      string `json:"created,omitempty"`
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Config       struct {
		User       string            `json:"User"`
		Entrypoint []string          `json:"Entrypoint"`
		Labels     map[string]string `json:"Labels,omitempty"`
	} `json:"config"`
	RootFS rootFS `json:"rootfs"`
}
type manifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
}
type index struct {
	SchemaVersion int          `json:"schemaVersion"`
	Manifests     []descriptor `json:"manifests"`
}

// Build writes an OCI Image Layout and returns the digest of its manifest.
func Build(opts Options) (string, error) {
	if err := normalize(&opts); err != nil {
		return "", err
	}
	info, err := os.Stat(opts.Binary)
	if err != nil {
		return "", fmt.Errorf("stat binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("binary must be a regular file")
	}
	if info.Mode()&0111 == 0 {
		return "", errors.New("binary is not executable")
	}
	if _, err := os.Stat(opts.Output); err == nil {
		return "", fmt.Errorf("output already exists: %s", opts.Output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat output: %w", err)
	}

	binary, err := os.ReadFile(opts.Binary)
	if err != nil {
		return "", fmt.Errorf("read binary: %w", err)
	}
	layer, diffID, err := makeLayer(binary, opts.Entrypoint)
	if err != nil {
		return "", err
	}
	layerDesc := newDescriptor(layerMediaType, layer)
	configBytes, err := makeConfig(opts, diffID)
	if err != nil {
		return "", err
	}
	configDesc := newDescriptor(configMediaType, configBytes)
	manifestBytes, err := json.Marshal(manifest{SchemaVersion: 2, Config: configDesc, Layers: []descriptor{layerDesc}})
	if err != nil {
		return "", fmt.Errorf("encode manifest: %w", err)
	}
	manifestDesc := newDescriptor(manifestMediaType, manifestBytes)
	manifestDesc.Platform = &platform{Architecture: opts.Architecture, OS: opts.OS}
	manifestDesc.Annotations = map[string]string{
		"org.opencontainers.image.ref.name": opts.ImageName + ":" + opts.Tag,
	}
	indexBytes, err := json.Marshal(index{SchemaVersion: 2, Manifests: []descriptor{manifestDesc}})
	if err != nil {
		return "", fmt.Errorf("encode index: %w", err)
	}

	tmp, err := os.MkdirTemp(filepath.Dir(opts.Output), ".oci-layout-")
	if err != nil {
		return "", fmt.Errorf("create temporary layout: %w", err)
	}
	defer os.RemoveAll(tmp)
	if err := writeLayout(tmp, indexBytes, [][]byte{layer, configBytes, manifestBytes}); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, opts.Output); err != nil {
		return "", fmt.Errorf("install layout: %w", err)
	}
	return manifestDesc.Digest, nil
}

func normalize(o *Options) error {
	if o.Binary == "" || o.Output == "" {
		return errors.New("binary and output are required")
	}
	if o.Architecture == "" {
		o.Architecture = runtime.GOARCH
	}
	if o.OS == "" {
		o.OS = "linux"
	}
	if o.Entrypoint == "" {
		o.Entrypoint = "/app/service"
	}
	if !strings.HasPrefix(o.Entrypoint, "/") || path.Clean(o.Entrypoint) != o.Entrypoint || o.Entrypoint == "/" {
		return errors.New("entrypoint must be an absolute, clean container path")
	}
	if o.Created.IsZero() {
		o.Created = time.Unix(0, 0).UTC()
	} else {
		o.Created = o.Created.UTC()
	}
	if o.ImageName == "" {
		o.ImageName = "secure-oci-base"
	}
	if o.Tag == "" {
		o.Tag = "latest"
	}
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	if err := os.MkdirAll(filepath.Dir(o.Output), 0755); err != nil {
		return fmt.Errorf("create output parent: %w", err)
	}
	return nil
}

func makeLayer(binary []byte, entrypoint string) ([]byte, string, error) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	name := strings.TrimPrefix(entrypoint, "/")
	parents := strings.Split(path.Dir(name), "/")
	current := ""
	for _, parent := range parents {
		if parent == "." {
			continue
		}
		current += parent + "/"
		if err := tw.WriteHeader(&tar.Header{Name: current, Typeflag: tar.TypeDir, Mode: 0755, ModTime: time.Unix(0, 0)}); err != nil {
			return nil, "", err
		}
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0555, Size: int64(len(binary)), ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR}); err != nil {
		return nil, "", err
	}
	if _, err := tw.Write(binary); err != nil {
		return nil, "", err
	}
	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	d := sha256.Sum256(raw.Bytes())
	var compressed bytes.Buffer
	gz, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return nil, "", err
	}
	gz.Header.ModTime = time.Unix(0, 0)
	gz.Header.OS = 255
	if _, err := gz.Write(raw.Bytes()); err != nil {
		return nil, "", err
	}
	if err := gz.Close(); err != nil {
		return nil, "", err
	}
	return compressed.Bytes(), "sha256:" + hex.EncodeToString(d[:]), nil
}

func makeConfig(o Options, diffID string) ([]byte, error) {
	var c imageConfig
	c.Created = o.Created.Format(time.RFC3339)
	c.Architecture = o.Architecture
	c.OS = o.OS
	c.Config.User = "65532:65532"
	c.Config.Entrypoint = []string{o.Entrypoint}
	c.Config.Labels = copyLabels(o.Labels)
	c.RootFS = rootFS{Type: "layers", DiffIDs: []string{diffID}}
	return json.Marshal(c)
}
func copyLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func newDescriptor(media string, data []byte) descriptor {
	d := sha256.Sum256(data)
	return descriptor{MediaType: media, Digest: "sha256:" + hex.EncodeToString(d[:]), Size: int64(len(data))}
}
func writeLayout(root string, idx []byte, blobs [][]byte) error {
	if err := os.MkdirAll(filepath.Join(root, "blobs", "sha256"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "oci-layout"), []byte("{\"imageLayoutVersion\":\"1.0.0\"}\n"), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "index.json"), append(idx, '\n'), 0644); err != nil {
		return err
	}
	for _, b := range blobs {
		d := sha256.Sum256(b)
		if err := os.WriteFile(filepath.Join(root, "blobs", "sha256", hex.EncodeToString(d[:])), b, 0644); err != nil {
			return err
		}
	}
	return nil
}

// LabelsFromPairs parses key=value labels and rejects ambiguous input.
func LabelsFromPairs(pairs []string) (map[string]string, error) {
	labels := make(map[string]string, len(pairs))
	sorted := append([]string(nil), pairs...)
	sort.Strings(sorted)
	for _, pair := range sorted {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("invalid label %q (expected key=value)", pair)
		}
		if _, exists := labels[k]; exists {
			return nil, fmt.Errorf("duplicate label %q", k)
		}
		labels[k] = v
	}
	return labels, nil
}

// Copy is retained for callers that need a standard-library stream copy.
func Copy(dst io.Writer, src io.Reader) (int64, error) { return io.Copy(dst, src) }
