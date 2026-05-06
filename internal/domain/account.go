package domain

import "time"

type Account struct {
	ID        string
	Name      string
	Bank      string
	Type      string
	Currency  string
	CreatedAt time.Time
}
