package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
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

func TestHealthCommandWithoutAkahuTokensPrintsOKAndSkipsAkahu(t *testing.T) {
	t.Setenv("DATABASE_URL_APP", "postgres://example")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("AKAHU_APP_TOKEN", "")
	t.Setenv("AKAHU_USER_TOKEN", "")

	pinged := false
	pingHealthDB = func(ctx context.Context) error {
		pinged = true
		return nil
	}
	checkAkahuHealth = func(ctx context.Context) error {
		t.Fatal("Akahu checker should not be called without tokens")
		return nil
	}
	t.Cleanup(func() {
		pingHealthDB = defaultPingHealthDB
		checkAkahuHealth = defaultCheckAkahuHealth
	})

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"health"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute health: %v", err)
	}
	if !pinged {
		t.Fatal("database was not pinged")
	}
	if got := out.String(); got != "ok\n" {
		t.Fatalf("output = %q, want ok", got)
	}
}

func TestHealthCommandWithAkahuTokensCallsChecker(t *testing.T) {
	t.Setenv("DATABASE_URL_APP", "postgres://example")
	t.Setenv("AKAHU_APP_TOKEN", "app_token_secret")
	t.Setenv("AKAHU_USER_TOKEN", "user_token_secret")

	pingHealthDB = func(ctx context.Context) error {
		return nil
	}
	called := false
	checkAkahuHealth = func(ctx context.Context) error {
		called = true
		return nil
	}
	t.Cleanup(func() {
		pingHealthDB = defaultPingHealthDB
		checkAkahuHealth = defaultCheckAkahuHealth
	})

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"health"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute health: %v", err)
	}
	if !called {
		t.Fatal("Akahu checker was not called")
	}
}

func TestHealthCommandWithAkahuTokensReturnsCheckerError(t *testing.T) {
	t.Setenv("DATABASE_URL_APP", "postgres://example")
	t.Setenv("AKAHU_APP_TOKEN", "app_token_secret")
	t.Setenv("AKAHU_USER_TOKEN", "user_token_secret")

	pingHealthDB = func(ctx context.Context) error {
		return nil
	}
	checkAkahuHealth = func(ctx context.Context) error {
		return errors.New("akahu unavailable")
	}
	t.Cleanup(func() {
		pingHealthDB = defaultPingHealthDB
		checkAkahuHealth = defaultCheckAkahuHealth
	})

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"health"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("health succeeded with failing Akahu checker")
	}
	if !strings.Contains(err.Error(), "akahu unavailable") {
		t.Fatalf("error = %q, want checker failure", err.Error())
	}
}

func TestHealthCommandCheckerFailureDoesNotLeakTokenValues(t *testing.T) {
	appToken := "app_token_secret"
	userToken := "user_token_secret"
	t.Setenv("DATABASE_URL_APP", "postgres://example")
	t.Setenv("AKAHU_APP_TOKEN", appToken)
	t.Setenv("AKAHU_USER_TOKEN", userToken)

	pingHealthDB = func(ctx context.Context) error {
		return nil
	}
	checkAkahuHealth = func(ctx context.Context) error {
		return errors.New("request failed with " + appToken + " and " + userToken)
	}
	t.Cleanup(func() {
		pingHealthDB = defaultPingHealthDB
		checkAkahuHealth = defaultCheckAkahuHealth
	})

	var out bytes.Buffer
	cmd := newRootCommand(&out, &out)
	cmd.SetArgs([]string{"health"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("health succeeded with failing Akahu checker")
	}
	if strings.Contains(err.Error(), appToken) || strings.Contains(err.Error(), userToken) {
		t.Fatalf("error leaked token values: %q", err.Error())
	}
}
