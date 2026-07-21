package main

import "testing"

func TestRunRequiresInputs(t *testing.T) {
	if code := Run(nil); code != 1 {
		t.Fatalf("Run() = %d, want 1", code)
	}
}
