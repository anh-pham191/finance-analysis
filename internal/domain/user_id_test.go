package domain

import "testing"

func TestNewUserIDRejectsNonPositiveValues(t *testing.T) {
	t.Parallel()

	for _, value := range []int64{0, -1} {
		if _, err := NewUserID(value); err == nil {
			t.Fatalf("NewUserID(%d) returned nil error", value)
		}
	}
}

func TestUserIDStringUsesDecimalRepresentation(t *testing.T) {
	t.Parallel()

	userID, err := NewUserID(42)
	if err != nil {
		t.Fatalf("NewUserID returned error: %v", err)
	}

	if got := userID.String(); got != "42" {
		t.Fatalf("UserID.String() = %q, want 42", got)
	}
}
