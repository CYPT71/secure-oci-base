package mtls

import "testing"

func TestConfig(t *testing.T) {
	c := New(false, "")
	if !c.Valid() {
		t.Fatalf("expected disabled config to be valid")
	}

	c = New(true, "")
	if c.Valid() {
		t.Fatalf("expected enabled config with empty cert to be invalid")
	}

	c = New(true, "cert-data")
	if !c.Valid() {
		t.Fatalf("expected enabled config with cert to be valid")
	}

	var nilCfg *Config
	if nilCfg.Valid() {
		t.Fatalf("expected nil config to be invalid")
	}
}
