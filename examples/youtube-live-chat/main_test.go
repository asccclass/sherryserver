package main

import "testing"

func TestValidateExternalWSURL(t *testing.T) {
	allowed := map[string]struct{}{"example.com": {}}
	if err := validateExternalWSURL("wss://example.com/ws", allowed); err != nil {
		t.Fatalf("expected URL to pass validation, got err: %v", err)
	}
	if err := validateExternalWSURL("http://example.com/ws", allowed); err == nil {
		t.Fatal("expected scheme validation error")
	}
	if err := validateExternalWSURL("wss://evil.com/ws", allowed); err == nil {
		t.Fatal("expected host allowlist validation error")
	}
}

func TestParseAllowedHosts(t *testing.T) {
	hosts := parseAllowedHosts("a.com, B.com ,,c.com")
	if len(hosts) != 3 {
		t.Fatalf("unexpected host count: %d", len(hosts))
	}
	if _, ok := hosts["b.com"]; !ok {
		t.Fatal("expected normalized lowercase host")
	}
}
