package main

import (
	"bytes"
	"testing"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	t.Parallel()

	version = "test-version"
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute version: %v", err)
	}
	if got := out.String(); got != "test-version\n" {
		t.Fatalf("version output = %q, want test-version newline", got)
	}
}
