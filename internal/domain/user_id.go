package domain

import (
	"fmt"
	"strconv"
)

type UserID int64

func NewUserID(value int64) (UserID, error) {
	if value <= 0 {
		return 0, fmt.Errorf("user id must be positive")
	}
	return UserID(value), nil
}

func (id UserID) Int64() int64 {
	return int64(id)
}

func (id UserID) String() string {
	return strconv.FormatInt(id.Int64(), 10)
}
