package main

import (
	"bytes"
	"testing"
)

func TestHealthCommandFailsWithoutDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL_APP", "")
	t.Setenv("DATABASE_URL", "")

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"health"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("health succeeded without database URL")
	}
}
