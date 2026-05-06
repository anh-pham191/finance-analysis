package observability

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactingHandlerRedactsTokenKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewTextHandler(&buf, nil)))
	logger.Info("token test", "app_token", "app_token_abc123456789")

	got := buf.String()
	if strings.Contains(got, "app_token_abc123456789") {
		t.Fatalf("log leaked token: %s", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("log = %q, want redacted marker", got)
	}
}

func TestRedactStringRemovesBearerTokens(t *testing.T) {
	t.Parallel()

	input := "Authorization: Bearer abc123def456ghi789"
	got := RedactString(input)
	if strings.Contains(got, "abc123def456ghi789") {
		t.Fatalf("RedactString leaked bearer token: %q", got)
	}
	if !strings.Contains(got, "Bearer ***") {
		t.Fatalf("RedactString = %q, want bearer redaction", got)
	}
}

func TestRedactingHandlerRedactsRecordMessage(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewTextHandler(&buf, nil)))
	logger.Info("request failed: Authorization: Bearer message_secret_123")

	got := buf.String()
	if strings.Contains(got, "message_secret_123") {
		t.Fatalf("log leaked token from message: %s", got)
	}
	if !strings.Contains(got, "Bearer ***") {
		t.Fatalf("log = %q, want bearer redaction", got)
	}
}

func TestRedactingHandlerRedactsAnyErrors(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewTextHandler(&buf, nil)))
	logger.Info("request failed", slog.Any("err", errors.New("upstream app_token_secret_123456789 failed")))

	got := buf.String()
	if strings.Contains(got, "app_token_secret_123456789") {
		t.Fatalf("log leaked token from error attr: %s", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("log = %q, want redacted marker", got)
	}
}

func TestRedactingHandlerRedactsAnyStringers(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(NewRedactingHandler(slog.NewTextHandler(&buf, nil)))
	logger.Info("request failed", slog.Any("response", stringerValue(`{"user_token":"user_token_secret_123"}`)))

	got := buf.String()
	if strings.Contains(got, "user_token_secret_123") {
		t.Fatalf("log leaked token from stringer attr: %s", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("log = %q, want redacted marker", got)
	}
}

func TestRedactStringRemovesJSONTokenAssignments(t *testing.T) {
	t.Parallel()

	got := RedactString(`{"user_token":"user_token_secret_123"}`)
	if strings.Contains(got, "user_token_secret_123") {
		t.Fatalf("RedactString leaked JSON token: %q", got)
	}
	if !strings.Contains(got, `"user_token":"***"`) {
		t.Fatalf("RedactString = %q, want JSON token redaction", got)
	}
}

func TestRedactStringRemovesStandaloneAkahuTokens(t *testing.T) {
	t.Parallel()

	got := RedactString("upstream returned app_token_secret_123456789")
	if strings.Contains(got, "app_token_secret_123456789") {
		t.Fatalf("RedactString leaked standalone Akahu token: %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("RedactString = %q, want redacted marker", got)
	}
}

func TestRedactStringRemovesLongBase64LikeValues(t *testing.T) {
	t.Parallel()

	secret := "abcDEF1234567890+/abcDEF1234567890"
	got := RedactString("body token=" + secret)
	if strings.Contains(got, secret) {
		t.Fatalf("RedactString leaked long base64-like value: %q", got)
	}
	if !strings.Contains(got, "***") {
		t.Fatalf("RedactString = %q, want redacted marker", got)
	}
}

type stringerValue string

func (s stringerValue) String() string {
	return string(s)
}
