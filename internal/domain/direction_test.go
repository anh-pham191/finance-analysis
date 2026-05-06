package domain

import "testing"

func TestParseDirectionAcceptsKnownValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  Direction
	}{
		{value: "DEBIT", want: DirectionDebit},
		{value: "CREDIT", want: DirectionCredit},
	}

	for _, tt := range tests {
		got, err := ParseDirection(tt.value)
		if err != nil {
			t.Fatalf("ParseDirection(%q) returned error: %v", tt.value, err)
		}
		if got != tt.want {
			t.Fatalf("ParseDirection(%q) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestParseDirectionRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	_, err := ParseDirection("TRANSFER")
	if err == nil {
		t.Fatal("ParseDirection accepted unknown direction")
	}
}
