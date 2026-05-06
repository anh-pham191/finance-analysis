package domain

import (
	"fmt"

	"github.com/shopspring/decimal"
)

type Money struct {
	value decimal.Decimal
}

func NewMoney(value decimal.Decimal) Money {
	return Money{value: value.Round(2)}
}

func NewMoneyFromString(value string) (Money, error) {
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return Money{}, fmt.Errorf("parse money: %w", err)
	}
	return NewMoney(amount), nil
}

func MustMoneyFromString(value string) Money {
	money, err := NewMoneyFromString(value)
	if err != nil {
		panic(err)
	}
	return money
}

func (m Money) Decimal() decimal.Decimal {
	return m.value
}

func (m Money) Add(other Money) Money {
	return NewMoney(m.value.Add(other.value))
}

func (m Money) String() string {
	return m.value.StringFixed(2)
}
