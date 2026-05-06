package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestSyncCommandParsesFromDate(t *testing.T) {
	var out bytes.Buffer
	var gotOptions syncOptions
	called := false
	syncRunner = func(ctx context.Context, opts syncOptions) error {
		called = true
		gotOptions = opts
		return nil
	}
	t.Cleanup(func() {
		syncRunner = runSync
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"sync", "--from", "2026-01-02"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute sync: %v", err)
	}
	if !called {
		t.Fatal("sync runner was not called")
	}
	if gotOptions.From == nil {
		t.Fatal("sync runner From = nil, want parsed date")
	}
	want := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if !gotOptions.From.Equal(want) {
		t.Fatalf("sync runner From = %v, want %v", gotOptions.From, want)
	}
}

func TestSyncCommandRejectsInvalidFromDate(t *testing.T) {
	var out bytes.Buffer
	syncRunner = func(ctx context.Context, opts syncOptions) error {
		t.Fatal("sync runner should not be called")
		return nil
	}
	t.Cleanup(func() {
		syncRunner = runSync
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"sync", "--from", "2026/01/02"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("sync command succeeded with invalid --from")
	}
	if !strings.Contains(err.Error(), "--from must be YYYY-MM-DD") {
		t.Fatalf("sync command error = %q, want --from format message", err.Error())
	}
}

func TestSyncCommandPassesNilFromWhenOmitted(t *testing.T) {
	var out bytes.Buffer
	called := false
	syncRunner = func(ctx context.Context, opts syncOptions) error {
		called = true
		if opts.From != nil {
			t.Fatalf("sync runner From = %v, want nil", opts.From)
		}
		return nil
	}
	t.Cleanup(func() {
		syncRunner = runSync
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"sync"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute sync: %v", err)
	}
	if !called {
		t.Fatal("sync runner was not called")
	}
}

func TestSyncCommandRejectsInvalidAkahuBaseURL(t *testing.T) {
	t.Setenv("AKAHU_APP_TOKEN", "app_token_secret")
	t.Setenv("AKAHU_USER_TOKEN", "user_token_secret")

	_, err := syncAkahuBaseURL("://bad-url")
	if err == nil {
		t.Fatal("invalid AKAHU_BASE_URL was accepted")
	}
	if !strings.Contains(err.Error(), "AKAHU_BASE_URL") {
		t.Fatalf("error = %q, want AKAHU_BASE_URL", err.Error())
	}
	if strings.Contains(err.Error(), "app_token_secret") || strings.Contains(err.Error(), "user_token_secret") {
		t.Fatalf("error leaked token values: %q", err.Error())
	}
}
