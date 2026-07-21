package oci

import "testing"

func TestBuilder(t *testing.T) {
	b := NewBuilder("example/image")
	got := b.Build("")
	want := "example/image:latest"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}

	got = b.Build("v1.2.3")
	want = "example/image:v1.2.3"
	if got != want {
		t.Fatalf("Build() = %q, want %q", got, want)
	}
}
