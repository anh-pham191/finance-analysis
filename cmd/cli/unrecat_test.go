package main

import (
	"bytes"
	"context"
	"testing"
)

func TestUnrecatCommandCallsRunner(t *testing.T) {
	var out bytes.Buffer
	called := false
	unrecatRunner = func(ctx context.Context, opts unrecatOptions) error {
		called = true
		if opts.TxnID != "txn-1" {
			t.Fatalf("txn ID = %q, want txn-1", opts.TxnID)
		}
		return nil
	}
	t.Cleanup(func() {
		unrecatRunner = runUnrecat
	})

	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"unrecat", "txn-1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute unrecat: %v", err)
	}
	if !called {
		t.Fatal("unrecat runner was not called")
	}
}

func TestUnrecatCommandRequiresTxn(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"unrecat"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("unrecat succeeded with missing args")
	}
}
