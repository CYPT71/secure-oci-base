package main

import "testing"

func TestMain(t *testing.T) {
	if code := Run(nil); code != 0 {
		t.Fatalf("Run() = %d, want 0", code)
	}
}
