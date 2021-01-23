package hcaptcha

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

const verifyURL = "https://hcaptcha.com/siteverify"

// Client is an hCaptcha client
type Client struct {
	ctx  context.Context
	http *http.Client

	secret string
}

// PostOptions are optional post form values
type PostOptions struct {
	RemoteIP string
	Sitekey  string
}

// ClientOption is a func to modify a new Client
type ClientOption func(*Client)

// WithHTTP sets the http.Client of a Client
func WithHTTP(httpClient *http.Client) func(*Client) {
	return func(hClient *Client) {
		hClient.http = httpClient
	}
}

// WithContext sets the context.Context of a Client
func WithContext(ctx context.Context) func(*Client) {
	return func(hClient *Client) {
		hClient.ctx = ctx
	}
}

// New returns a new hCaptcha Client
func New(secret string, options ...ClientOption) (*Client, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, ErrMissingInputSecret
	}

	client := &Client{
		ctx:    context.Background(),
		http:   http.DefaultClient,
		secret: secret,
	}

	for _, opt := range options {
		opt(client)
	}

	return client, nil
}

// Verify checks the response against the hCaptcha API
func (c *Client) Verify(token string, opts PostOptions) (*Response, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrMissingInputResponse
	}

	post := url.Values{
		"secret":   []string{c.secret},
		"response": []string{token},
	}
	if strings.TrimSpace(opts.RemoteIP) != "" {
		post.Add("remoteip", opts.RemoteIP)
	}
	if strings.TrimSpace(opts.Sitekey) != "" {
		post.Add("sitekey", opts.Sitekey)
	}

	// Basically a copy of http.PostForm, but with a context
	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, verifyURL, strings.NewReader(post.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response *Response
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return response, nil
}
