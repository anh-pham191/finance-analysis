package akahu

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestDoWithRetryRetries429ThenSucceeds(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	attempts := 0
	policy := testRetryPolicy(&sleeps)

	resp, err := policy.do(context.Background(), func(context.Context) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return responseWithStatus(http.StatusTooManyRequests), nil
		}
		return responseWithStatus(http.StatusOK), nil
	})

	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if want := []time.Duration{time.Second, 2 * time.Second}; !durationsEqual(sleeps, want) {
		t.Fatalf("sleeps = %v, want %v", sleeps, want)
	}
}

func TestDoWithRetryReturnsErrorAfterMaxRetries(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	attempts := 0
	policy := testRetryPolicy(&sleeps)

	resp, err := policy.do(context.Background(), func(context.Context) (*http.Response, error) {
		attempts++
		return responseWithStatus(http.StatusInternalServerError), nil
	})

	if err == nil {
		t.Fatal("retry returned nil error, want error")
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if attempts != 4 {
		t.Fatalf("attempts = %d, want 4", attempts)
	}
	if want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}; !durationsEqual(sleeps, want) {
		t.Fatalf("sleeps = %v, want %v", sleeps, want)
	}
}

func TestDoWithRetryDoesNotRetryNon429ClientError(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	attempts := 0
	policy := testRetryPolicy(&sleeps)

	resp, err := policy.do(context.Background(), func(context.Context) (*http.Response, error) {
		attempts++
		return responseWithStatus(http.StatusBadRequest), nil
	})

	if err == nil {
		t.Fatal("retry returned nil error, want error")
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	if len(sleeps) != 0 {
		t.Fatalf("sleeps = %v, want none", sleeps)
	}
}

func TestDoWithRetryHonoursRetryAfterSeconds(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	attempts := 0
	policy := testRetryPolicy(&sleeps)

	resp, err := policy.do(context.Background(), func(context.Context) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			resp := responseWithStatus(http.StatusTooManyRequests)
			resp.Header.Set("Retry-After", "2")
			return resp, nil
		}
		return responseWithStatus(http.StatusOK), nil
	})

	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if want := []time.Duration{2 * time.Second}; !durationsEqual(sleeps, want) {
		t.Fatalf("sleeps = %v, want %v", sleeps, want)
	}
}

func TestDoWithRetryStopsWhenContextCancelledDuringSleep(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	policy := retryPolicy{
		maxRetries: 3,
		baseDelay:  time.Second,
		jitter:     func(time.Duration) time.Duration { return 0 },
		sleep: func(context.Context, time.Duration) error {
			cancel()
			return ctx.Err()
		},
	}

	resp, err := policy.do(ctx, func(context.Context) (*http.Response, error) {
		attempts++
		return responseWithStatus(http.StatusTooManyRequests), nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestDoWithRetryRetriesNetworkErrorsUntilSuccess(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	attempts := 0
	policy := testRetryPolicy(&sleeps)

	resp, err := policy.do(context.Background(), func(context.Context) (*http.Response, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("connection reset")
		}
		return responseWithStatus(http.StatusOK), nil
	})

	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if want := []time.Duration{time.Second, 2 * time.Second}; !durationsEqual(sleeps, want) {
		t.Fatalf("sleeps = %v, want %v", sleeps, want)
	}
}

func TestDoWithRetryClosesRetryableResponseBodies(t *testing.T) {
	t.Parallel()

	var sleeps []time.Duration
	body := &trackingBody{}
	attempts := 0
	policy := testRetryPolicy(&sleeps)

	resp, err := policy.do(context.Background(), func(context.Context) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     make(http.Header),
				Body:       body,
			}, nil
		}
		return responseWithStatus(http.StatusOK), nil
	})

	if err != nil {
		t.Fatalf("retry returned error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !body.closed {
		t.Fatal("retryable response body was not closed")
	}
}

func testRetryPolicy(sleeps *[]time.Duration) retryPolicy {
	return retryPolicy{
		maxRetries: 3,
		baseDelay:  time.Second,
		jitter:     func(time.Duration) time.Duration { return 0 },
		sleep: func(ctx context.Context, delay time.Duration) error {
			*sleeps = append(*sleeps, delay)
			return nil
		},
	}
}

func responseWithStatus(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
	}
}

func durationsEqual(got, want []time.Duration) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

type trackingBody struct {
	closed bool
}

func (b *trackingBody) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (b *trackingBody) Close() error {
	b.closed = true
	return nil
}
