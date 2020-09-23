package gitlab

import (
	"net/http"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// ClientOptionFunc can be used customize a new GitLab API client.
type ClientOptionFunc func(*Client) error

// WithBaseURL sets the base URL for API requests to a custom endpoint.
func WithBaseURL(urlStr string) ClientOptionFunc {
	return func(c *Client) error {
		return c.setBaseURL(urlStr)
	}
}

// WithCustomBackoff can be used to configure a custom backoff policy.
func WithCustomBackoff(backoff retryablehttp.Backoff) ClientOptionFunc {
	return func(c *Client) error {
		c.client.Backoff = backoff
		return nil
	}
}

// WithCustomLimiter injects a custom rate limiter to the client.
func WithCustomLimiter(limiter RateLimiter) ClientOptionFunc {
	return func(c *Client) error {
		c.configureLimiterOnce.Do(func() {
			c.limiter = limiter
		})
		return nil
	}
}

// WithCustomRetry can be used to configure a custom retry policy.
func WithCustomRetry(checkRetry retryablehttp.CheckRetry) ClientOptionFunc {
	return func(c *Client) error {
		c.client.CheckRetry = checkRetry
		return nil
	}
}

// WithHTTPClient can be used to configure a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOptionFunc {
	return func(c *Client) error {
		c.client.HTTPClient = httpClient
		return nil
	}
}

// WithoutRetries disables the default retry logic.
func WithoutRetries() ClientOptionFunc {
	return func(c *Client) error {
		c.disableRetries = true
		return nil
	}
}
