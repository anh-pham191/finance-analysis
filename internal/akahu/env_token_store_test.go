package akahu

import (
	"context"
	"strings"
	"testing"

	"github.com/anh-pham191/finance-analysis/internal/domain"
)

func TestEnvTokenStoreReturnsErrorWhenAppTokenMissing(t *testing.T) {
	t.Setenv("AKAHU_APP_TOKEN", "")
	t.Setenv("AKAHU_USER_TOKEN", "user_token_test_value")

	_, _, err := EnvTokenStore{}.AkahuTokens(context.Background(), domain.UserID(1))
	if err == nil {
		t.Fatal("AkahuTokens returned nil error, want error")
	}
	if strings.Contains(err.Error(), "user_token_test_value") {
		t.Fatalf("error = %q, want no token values", err.Error())
	}
}

func TestEnvTokenStoreReturnsErrorWhenUserTokenMissing(t *testing.T) {
	t.Setenv("AKAHU_APP_TOKEN", "app_token_test_value")
	t.Setenv("AKAHU_USER_TOKEN", "")

	_, _, err := EnvTokenStore{}.AkahuTokens(context.Background(), domain.UserID(1))
	if err == nil {
		t.Fatal("AkahuTokens returned nil error, want error")
	}
	if strings.Contains(err.Error(), "app_token_test_value") {
		t.Fatalf("error = %q, want no token values", err.Error())
	}
}

func TestEnvTokenStoreReturnsBothConfiguredTokens(t *testing.T) {
	t.Setenv("AKAHU_APP_TOKEN", "app_token_test_value")
	t.Setenv("AKAHU_USER_TOKEN", "user_token_test_value")

	appToken, userToken, err := EnvTokenStore{}.AkahuTokens(context.Background(), domain.UserID(1))
	if err != nil {
		t.Fatalf("AkahuTokens returned error: %v", err)
	}
	if appToken != "app_token_test_value" {
		t.Fatalf("app token = %q, want configured value", appToken)
	}
	if userToken != "user_token_test_value" {
		t.Fatalf("user token = %q, want configured value", userToken)
	}
}

func TestEnvTokenStoreReturnsSameTokensForDifferentUsersIntentionalM2Weakening(t *testing.T) {
	t.Setenv("AKAHU_APP_TOKEN", "app_token_test_value")
	t.Setenv("AKAHU_USER_TOKEN", "user_token_test_value")

	firstApp, firstUser, err := EnvTokenStore{}.AkahuTokens(context.Background(), domain.UserID(1))
	if err != nil {
		t.Fatalf("AkahuTokens for first user returned error: %v", err)
	}
	secondApp, secondUser, err := EnvTokenStore{}.AkahuTokens(context.Background(), domain.UserID(2))
	if err != nil {
		t.Fatalf("AkahuTokens for second user returned error: %v", err)
	}

	if firstApp != secondApp {
		t.Fatalf("app tokens differ: first %q, second %q", firstApp, secondApp)
	}
	if firstUser != secondUser {
		t.Fatalf("user tokens differ: first %q, second %q", firstUser, secondUser)
	}
}
