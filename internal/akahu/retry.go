package akahu

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type retryPolicy struct {
	maxRetries int
	baseDelay  time.Duration
	jitter     func(time.Duration) time.Duration
	sleep      func(context.Context, time.Duration) error
}

func (p retryPolicy) do(ctx context.Context, fn func(context.Context) (*http.Response, error)) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, err := fn(ctx)
		if err == nil && !isRetryableStatus(resp.StatusCode) {
			if resp.StatusCode >= http.StatusBadRequest {
				closeResponseBody(resp)
				return nil, fmt.Errorf("akahu request failed with status %d", resp.StatusCode)
			}
			return resp, nil
		}

		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("akahu request failed with status %d", resp.StatusCode)
		}

		if attempt == p.maxRetries {
			closeResponseBody(resp)
			return nil, lastErr
		}

		delay := p.retryDelay(attempt, resp)
		closeResponseBody(resp)
		if err := p.sleep(ctx, delay); err != nil {
			return nil, err
		}
	}

	return nil, lastErr
}

func (p retryPolicy) retryDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if delay, ok := parseRetryAfter(resp.Header.Get("Retry-After")); ok {
			return delay
		}
	}

	delay := p.baseDelay << attempt
	if p.jitter != nil {
		delay += p.jitter(delay)
	}
	return delay
}

func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func parseRetryAfter(value string) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}

	seconds, err := strconv.Atoi(value)
	if err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}

	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	delay := time.Until(when)
	if delay < 0 {
		return 0, true
	}
	return delay, true
}

func closeResponseBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}
