package akahu

import (
	"encoding/json"
)

type cursorResponse struct {
	Next string `json:"next"`
}

type accountListResponse struct {
	Items  []accountResponse `json:"items"`
	Cursor cursorResponse    `json:"cursor"`
}

type accountResponse struct {
	ID         string `json:"_id"`
	Name       string `json:"name"`
	Connection struct {
		Name string `json:"name"`
	} `json:"connection"`
	Type    string `json:"type"`
	Balance struct {
		Currency string `json:"currency"`
	} `json:"balance"`
}

type transactionListResponse struct {
	Items  []json.RawMessage `json:"items"`
	Cursor cursorResponse    `json:"cursor"`
}

type transactionResponse struct {
	ID          string          `json:"_id"`
	AccountID   string          `json:"_account"`
	Date        string          `json:"date"`
	Amount      json.RawMessage `json:"amount"`
	Direction   string          `json:"type"`
	Description string          `json:"description"`
	Merchant    struct {
		Name string `json:"name"`
	} `json:"merchant"`
	Category struct {
		Name string `json:"name"`
	} `json:"category"`
}
