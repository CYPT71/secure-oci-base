package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWithoutArgumentsShowsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run(nil, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), "-binary") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRunArgumentErrors(t *testing.T) {
	for _, args := range [][]string{{"-created", "bad"}, {"-label", "bad"}, {"-unknown"}} {
		var out, err bytes.Buffer
		if code := run(args, &out, &err); code != 2 {
			t.Fatalf("run(%v) = %d, stderr=%s", args, code, err.String())
		}
	}
}

func TestRunBuildsLayout(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "service")
	if err := os.WriteFile(binary, []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := run([]string{"-binary", binary, "-output", filepath.Join(dir, "layout"), "-arch", "amd64", "-label", "a=b"}, &out, &stderr)
	if code != 0 || !strings.Contains(out.String(), "created OCI layout") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), stderr.String())
	}
	var labels labelFlags
	if err := labels.Set("a=b"); err != nil || labels.String() != "" {
		t.Fatal("label flag failed")
	}
}
