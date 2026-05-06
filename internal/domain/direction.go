package domain

import "fmt"

type Direction string

const (
	DirectionDebit  Direction = "DEBIT"
	DirectionCredit Direction = "CREDIT"
)

func ParseDirection(value string) (Direction, error) {
	switch Direction(value) {
	case DirectionDebit:
		return DirectionDebit, nil
	case DirectionCredit:
		return DirectionCredit, nil
	default:
		return "", fmt.Errorf("unknown direction %q", value)
	}
}
