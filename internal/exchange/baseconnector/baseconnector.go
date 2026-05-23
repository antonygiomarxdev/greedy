package baseconnector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/ratelimiter"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

const defaultMaxRetries = 3

type BaseConnector struct {
	Client     *http.Client
	RL         ratelimiter.RateLimiter
	MaxRetries int
}

type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    io.Reader
}

type Response struct {
	Body       []byte
	StatusCode int
	Header     http.Header
}

func (b *BaseConnector) Do(ctx context.Context, req *Request) (*Response, error) {
	if err := b.RL.Wait(ctx); err != nil {
		return nil, err
	}

	maxRetries := b.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		r, err := http.NewRequestWithContext(ctx, req.Method, req.URL, req.Body)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		for k, v := range req.Headers {
			r.Header.Set(k, v)
		}

		resp, err := b.Client.Do(r)
		if err != nil {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("http request: %w", err)
			}
			time.Sleep(time.Duration(attempt+1) * 100 * time.Millisecond)
			continue
		}

		data, readErr := io.ReadAll(resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil && readErr == nil {
			return nil, fmt.Errorf("close response: %w", closeErr)
		}
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
			if attempt == maxRetries-1 {
				return nil, shared.ErrRateLimited
			}
			retryAfter := resp.Header.Get("Retry-After")
			if d, err := strconv.Atoi(retryAfter); err == nil {
				time.Sleep(time.Duration(d) * time.Second)
			} else {
				time.Sleep(time.Duration(attempt+1) * time.Second)
			}
			continue
		}

		return &Response{
			Body:       data,
			StatusCode: resp.StatusCode,
			Header:     resp.Header,
		}, nil
	}

	return nil, shared.ErrExchangeDown
}
