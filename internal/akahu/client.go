package akahu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anh-pham191/finance-analysis/internal/observability"
	"github.com/anh-pham191/finance-analysis/internal/ports"
)

const maxErrorBodyBytes = 4096

type Config struct {
	AppToken   string
	UserToken  string
	BaseURL    string
	HTTPClient *http.Client
}

type Client struct {
	appToken   string
	userToken  string
	baseURL    *url.URL
	httpClient *http.Client
	retry      retryPolicy
}

func NewClient(config Config) *Client {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	baseURL, _ := url.Parse(strings.TrimRight(config.BaseURL, "/"))

	return &Client{
		appToken:   config.AppToken,
		userToken:  config.UserToken,
		baseURL:    baseURL,
		httpClient: httpClient,
		retry: retryPolicy{
			maxRetries: 3,
			baseDelay:  100 * time.Millisecond,
			jitter:     func(time.Duration) time.Duration { return 0 },
			sleep: func(ctx context.Context, delay time.Duration) error {
				timer := time.NewTimer(delay)
				defer timer.Stop()

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-timer.C:
					return nil
				}
			},
		},
	}
}

func (c *Client) ListAccounts(ctx context.Context) ([]ports.RawAccount, error) {
	var accounts []ports.RawAccount
	err := c.eachPage(ctx, "list accounts", c.endpoint("/accounts"), func(body []byte) (string, error) {
		var response accountListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return "", fmt.Errorf("akahu list accounts: decode response: %w", err)
		}

		for _, item := range response.Items {
			accounts = append(accounts, ports.RawAccount{
				ID:       item.ID,
				Name:     item.Name,
				Bank:     item.Connection.Name,
				Type:     item.Type,
				Currency: item.Balance.Currency,
			})
		}
		return response.Cursor.Next, nil
	})
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func (c *Client) FetchTransactions(ctx context.Context, accountID string, since time.Time) ([]ports.RawTxn, error) {
	path := fmt.Sprintf("/accounts/%s/transactions", url.PathEscape(accountID))
	endpoint := c.endpoint(path)
	query := endpoint.Query()
	query.Set("since", since.Format(time.RFC3339))
	endpoint.RawQuery = query.Encode()

	var txns []ports.RawTxn
	err := c.eachPage(ctx, "fetch transactions", endpoint, func(body []byte) (string, error) {
		var response transactionListResponse
		if err := json.Unmarshal(body, &response); err != nil {
			return "", fmt.Errorf("akahu fetch transactions: decode response: %w", err)
		}

		for _, raw := range response.Items {
			txn, err := mapTransaction(raw)
			if err != nil {
				return "", err
			}
			txns = append(txns, txn)
		}
		return response.Cursor.Next, nil
	})
	if err != nil {
		return nil, err
	}
	return txns, nil
}

func (c *Client) eachPage(ctx context.Context, operation string, firstURL *url.URL, handle func([]byte) (string, error)) error {
	currentURL := firstURL
	for {
		body, err := c.get(ctx, operation, currentURL)
		if err != nil {
			return err
		}

		next, err := handle(body)
		if err != nil {
			return err
		}
		if next == "" {
			return nil
		}

		currentURL, err = c.nextURL(currentURL, next)
		if err != nil {
			return fmt.Errorf("akahu %s: invalid pagination cursor", operation)
		}
	}
}

func (c *Client) get(ctx context.Context, operation string, endpoint *url.URL) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retry.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("akahu %s: build request: %s", operation, observability.RedactString(err.Error()))
		}
		req.Header.Set("Authorization", "Bearer "+c.userToken)
		req.Header.Set("X-Akahu-ID", c.appToken)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("akahu %s: request failed: %s", operation, observability.RedactString(err.Error()))
		} else if isRetryableStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("akahu %s failed with status %d", operation, resp.StatusCode)
		} else {
			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
				closeResponseBody(resp)
				if readErr != nil {
					return nil, fmt.Errorf("akahu %s: read response: %w", operation, readErr)
				}
				_ = observability.RedactString(string(bytes.TrimSpace(body)))
				return nil, fmt.Errorf("akahu %s failed with status %d", operation, resp.StatusCode)
			}
			body, readErr := io.ReadAll(resp.Body)
			closeResponseBody(resp)
			if readErr != nil {
				return nil, fmt.Errorf("akahu %s: read response: %w", operation, readErr)
			}
			return body, nil
		}

		if attempt == c.retry.maxRetries {
			closeResponseBody(resp)
			return nil, lastErr
		}

		delay := c.retry.retryDelay(attempt, resp)
		closeResponseBody(resp)
		if err := c.retry.sleep(ctx, delay); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func (c *Client) endpoint(path string) *url.URL {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + path
	endpoint.RawQuery = ""
	return &endpoint
}

func (c *Client) nextURL(currentURL *url.URL, next string) (*url.URL, error) {
	parsed, err := url.Parse(next)
	if err != nil {
		return nil, err
	}
	if parsed.IsAbs() {
		return parsed, nil
	}
	return currentURL.ResolveReference(parsed), nil
}

func mapTransaction(raw json.RawMessage) (ports.RawTxn, error) {
	var item transactionResponse
	if err := json.Unmarshal(raw, &item); err != nil {
		return ports.RawTxn{}, fmt.Errorf("akahu fetch transactions: decode transaction: %w", err)
	}

	postedAt, err := time.Parse(time.RFC3339, item.Date)
	if err != nil {
		return ports.RawTxn{}, fmt.Errorf("akahu fetch transactions: transaction %s invalid date", item.ID)
	}

	amount, err := amountString(item.Amount)
	if err != nil {
		return ports.RawTxn{}, fmt.Errorf("akahu fetch transactions: transaction %s invalid amount", item.ID)
	}

	rawCopy := append(json.RawMessage(nil), raw...)
	return ports.RawTxn{
		ID:            item.ID,
		AccountID:     item.AccountID,
		PostedAt:      postedAt,
		Amount:        amount,
		Direction:     item.Direction,
		Description:   item.Description,
		Merchant:      item.Merchant.Name,
		AkahuCategory: item.Category.Name,
		RawJSON:       rawCopy,
	}, nil
}

func amountString(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	var asNumber json.Number
	decoder := json.NewDecoder(strings.NewReader(trimmed))
	decoder.UseNumber()
	if err := decoder.Decode(&asNumber); err != nil || !json.Valid(raw) {
		return "", fmt.Errorf("invalid JSON amount")
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", fmt.Errorf("invalid JSON amount")
	}
	return asNumber.String(), nil
}
