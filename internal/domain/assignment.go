package domain

import "time"

type AssignmentSource string

const (
	AssignmentSourceRule   AssignmentSource = "RULE"
	AssignmentSourceManual AssignmentSource = "MANUAL"
	AssignmentSourceAkahu  AssignmentSource = "AKAHU"
)

type CategoryAssignment struct {
	TxnID      string
	CategoryID int64
	Source     AssignmentSource
	RuleID     *int64
	AssignedAt time.Time
}
