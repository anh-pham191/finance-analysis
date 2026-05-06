package domain

import "testing"

func TestNewMoneyFromStringPreservesTwoDecimalPlaces(t *testing.T) {
	t.Parallel()

	money, err := NewMoneyFromString("12.30")
	if err != nil {
		t.Fatalf("NewMoneyFromString returned error: %v", err)
	}

	if got := money.String(); got != "12.30" {
		t.Fatalf("Money.String() = %q, want 12.30", got)
	}
}

func TestMoneyAddReturnsSum(t *testing.T) {
	t.Parallel()

	left := MustMoneyFromString("10.25")
	right := MustMoneyFromString("2.05")

	if got := left.Add(right).String(); got != "12.30" {
		t.Fatalf("Money.Add() = %q, want 12.30", got)
	}
}
