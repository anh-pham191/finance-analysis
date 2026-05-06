package domain

import (
	"encoding/json"
	"time"
)

type Rule struct {
	ID         int64
	Name       string
	Priority   int
	Predicate  json.RawMessage
	CategoryID int64
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
