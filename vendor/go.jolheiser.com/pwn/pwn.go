package pwn

import (
	"context"
	"io"
	"net/http"
)

const (
	libVersion = "0.0.3"
	userAgent  = "go.jolheiser.com/pwn v" + libVersion
)

// Client is a HaveIBeenPwned client
type Client struct {
	ctx  context.Context
	http *http.Client
}

// New returns a new HaveIBeenPwned Client
func New(options ...ClientOption) *Client {
	client := &Client{
		ctx:  context.Background(),
		http: http.DefaultClient,
	}

	for _, opt := range options {
		opt(client)
	}

	return client
}

// ClientOption is a way to modify a new Client
type ClientOption func(*Client)

// WithHTTP will set the http.Client of a Client
func WithHTTP(httpClient *http.Client) func(pwnClient *Client) {
	return func(pwnClient *Client) {
		pwnClient.http = httpClient
	}
}

// WithContext will set the context.Context of a Client
func WithContext(ctx context.Context) func(pwnClient *Client) {
	return func(pwnClient *Client) {
		pwnClient.ctx = ctx
	}
}

func newRequest(ctx context.Context, method, url string, body io.ReadCloser) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", userAgent)
	return req, nil
}
