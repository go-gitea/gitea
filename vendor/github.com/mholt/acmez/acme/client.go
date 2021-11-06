// Copyright 2020 Matthew Holt
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package acme fully implements the ACME protocol specification as
// described in RFC 8555: https://tools.ietf.org/html/rfc8555.
//
// It is designed to work smoothly in large-scale deployments with
// high resilience to errors and intermittent network or server issues,
// with retries built-in at every layer of the HTTP request stack.
//
// NOTE: This is a low-level API. Most users will want the mholt/acmez
// package which is more concerned with configuring challenges and
// implementing the order flow. However, using this package directly
// is recommended for advanced use cases having niche requirements.
// See the examples in the examples/plumbing folder for a tutorial.
package acme

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Client facilitates ACME client operations as defined by the spec.
//
// Because the client is synchronized for concurrent use, it should
// not be copied.
//
// Many errors that are returned by a Client are likely to be of type
// Problem as long as the ACME server returns a structured error
// response. This package wraps errors that may be of type Problem,
// so you can access the details using the conventional Go pattern:
//
//     var problem Problem
//     if errors.As(err, &problem) {
//         log.Printf("Houston, we have a problem: %+v", problem)
//     }
//
// All Problem errors originate from the ACME server.
type Client struct {
	// The ACME server's directory endpoint.
	Directory string

	// Custom HTTP client.
	HTTPClient *http.Client

	// Augmentation of the User-Agent header. Please set
	// this so that CAs can troubleshoot bugs more easily.
	UserAgent string

	// Delay between poll attempts. Only used if server
	// does not supply a Retry-Afer header. Default: 250ms
	PollInterval time.Duration

	// Maximum duration for polling. Default: 5m
	PollTimeout time.Duration

	// An optional logger. Default: no logs
	Logger *zap.Logger

	mu     sync.Mutex // protects all unexported fields
	dir    Directory
	nonces *stack
}

// GetDirectory retrieves the directory configured at c.Directory. It is
// NOT necessary to call this to provision the client. It is only useful
// if you want to access a copy of the directory yourself.
func (c *Client) GetDirectory(ctx context.Context) (Directory, error) {
	if err := c.provision(ctx); err != nil {
		return Directory{}, err
	}
	return c.dir, nil
}

func (c *Client) provision(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nonces == nil {
		c.nonces = new(stack)
	}

	err := c.provisionDirectory(ctx)
	if err != nil {
		return fmt.Errorf("provisioning client: %w", err)
	}

	return nil
}

func (c *Client) provisionDirectory(ctx context.Context) error {
	// don't get directory again if we already have it;
	// checking any one of the required fields will do
	if c.dir.NewNonce != "" {
		return nil
	}
	if c.Directory == "" {
		return fmt.Errorf("missing directory URL")
	}
	// prefer cached version if it's recent enough
	directoriesMu.Lock()
	defer directoriesMu.Unlock()
	if dir, ok := directories[c.Directory]; ok {
		if time.Since(dir.retrieved) < 12*time.Hour {
			c.dir = dir.Directory
			return nil
		}
	}
	_, err := c.httpReq(ctx, http.MethodGet, c.Directory, nil, &c.dir)
	if err != nil {
		return err
	}
	directories[c.Directory] = cachedDirectory{c.dir, time.Now()}
	return nil
}

func (c *Client) nonce(ctx context.Context) (string, error) {
	nonce := c.nonces.pop()
	if nonce != "" {
		return nonce, nil
	}

	if c.dir.NewNonce == "" {
		return "", fmt.Errorf("directory missing newNonce endpoint")
	}

	resp, err := c.httpReq(ctx, http.MethodHead, c.dir.NewNonce, nil, nil)
	if err != nil {
		return "", fmt.Errorf("fetching new nonce from server: %w", err)
	}

	return resp.Header.Get(replayNonce), nil
}

func (c *Client) pollInterval() time.Duration {
	if c.PollInterval == 0 {
		return defaultPollInterval
	}
	return c.PollInterval
}

func (c *Client) pollTimeout() time.Duration {
	if c.PollTimeout == 0 {
		return defaultPollTimeout
	}
	return c.PollTimeout
}

// Directory acts as an index for the ACME server as
// specified in the spec: "In order to help clients
// configure themselves with the right URLs for each
// ACME operation, ACME servers provide a directory
// object." §7.1.1
type Directory struct {
	NewNonce   string         `json:"newNonce"`
	NewAccount string         `json:"newAccount"`
	NewOrder   string         `json:"newOrder"`
	NewAuthz   string         `json:"newAuthz,omitempty"`
	RevokeCert string         `json:"revokeCert"`
	KeyChange  string         `json:"keyChange"`
	Meta       *DirectoryMeta `json:"meta,omitempty"`
}

// DirectoryMeta is optional extra data that may be
// included in an ACME server directory. §7.1.1
type DirectoryMeta struct {
	TermsOfService          string   `json:"termsOfService,omitempty"`
	Website                 string   `json:"website,omitempty"`
	CAAIdentities           []string `json:"caaIdentities,omitempty"`
	ExternalAccountRequired bool     `json:"externalAccountRequired,omitempty"`
}

// stack is a simple thread-safe stack.
type stack struct {
	stack   []string
	stackMu sync.Mutex
}

func (s *stack) push(v string) {
	if v == "" {
		return
	}
	s.stackMu.Lock()
	defer s.stackMu.Unlock()
	if len(s.stack) >= 64 {
		return
	}
	s.stack = append(s.stack, v)
}

func (s *stack) pop() string {
	s.stackMu.Lock()
	defer s.stackMu.Unlock()
	n := len(s.stack)
	if n == 0 {
		return ""
	}
	v := s.stack[n-1]
	s.stack = s.stack[:n-1]
	return v
}

// Directories seldom (if ever) change in practice, and
// client structs are often ephemeral, so we can cache
// directories to speed things up a bit for the user.
// Keyed by directory URL.
var (
	directories   = make(map[string]cachedDirectory)
	directoriesMu sync.Mutex
)

type cachedDirectory struct {
	Directory
	retrieved time.Time
}

// replayNonce is the header field that contains a new
// anti-replay nonce from the server.
const replayNonce = "Replay-Nonce"

const (
	defaultPollInterval = 250 * time.Millisecond
	defaultPollTimeout  = 5 * time.Minute
)
