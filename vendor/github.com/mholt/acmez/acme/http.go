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

package acme

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// httpPostJWS performs robust HTTP requests by JWS-encoding the JSON of input.
// If output is specified, the response body is written into it: if the response
// Content-Type is JSON, it will be JSON-decoded into output (which must be a
// pointer); otherwise, if output is an io.Writer, the response body will be
// written to it uninterpreted. In all cases, the returned response value's
// body will have been drained and closed, so there is no need to close it again.
// It automatically retries in the case of network, I/O, or badNonce errors.
func (c *Client) httpPostJWS(ctx context.Context, privateKey crypto.Signer,
	kid, endpoint string, input, output interface{}) (*http.Response, error) {

	if err := c.provision(ctx); err != nil {
		return nil, err
	}

	var resp *http.Response
	var err error

	// we can retry on internal server errors just in case it was a hiccup,
	// but we probably don't need to retry so many times in that case
	internalServerErrors, maxInternalServerErrors := 0, 3

	// set a hard cap on the number of retries for any other reason
	const maxAttempts = 10
	var attempts int
	for attempts = 1; attempts <= maxAttempts; attempts++ {
		if attempts > 1 {
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		var nonce string // avoid shadowing err
		nonce, err = c.nonce(ctx)
		if err != nil {
			return nil, err
		}

		var encodedPayload []byte // avoid shadowing err
		encodedPayload, err = jwsEncodeJSON(input, privateKey, keyID(kid), nonce, endpoint)
		if err != nil {
			return nil, fmt.Errorf("encoding payload: %v", err)
		}

		resp, err = c.httpReq(ctx, http.MethodPost, endpoint, encodedPayload, output)
		if err == nil {
			return resp, nil
		}

		// "When a server rejects a request because its nonce value was
		// unacceptable (or not present), it MUST provide HTTP status code 400
		// (Bad Request), and indicate the ACME error type
		// 'urn:ietf:params:acme:error:badNonce'.  An error response with the
		// 'badNonce' error type MUST include a Replay-Nonce header field with a
		// fresh nonce that the server will accept in a retry of the original
		// query (and possibly in other requests, according to the server's
		// nonce scoping policy).  On receiving such a response, a client SHOULD
		// retry the request using the new nonce." §6.5
		var problem Problem
		if errors.As(err, &problem) {
			if problem.Type == ProblemTypeBadNonce {
				if c.Logger != nil {
					c.Logger.Debug("server rejected our nonce; retrying",
						zap.String("detail", problem.Detail),
						zap.Error(err))
				}
				continue
			}
		}

		// internal server errors *could* just be a hiccup and it may be worth
		// trying again, but not nearly so many times as for other reasons
		if resp != nil && resp.StatusCode >= 500 {
			internalServerErrors++
			if internalServerErrors < maxInternalServerErrors {
				continue
			}
		}

		// for any other error, there's not much we can do automatically
		break
	}

	return resp, fmt.Errorf("attempt %d: %s: %w", attempts, endpoint, err)
}

// httpReq robustly performs an HTTP request using the given method to the given endpoint, honoring
// the given context's cancellation. The joseJSONPayload is optional; if not nil, it is expected to
// be a JOSE+JSON encoding. The output is also optional; if not nil, the response body will be read
// into output. If the response Content-Type is JSON, it will be JSON-decoded into output, which
// must be a pointer type. If the response is any other Content-Type and if output is a io.Writer,
// it will be written (without interpretation or decoding) to output. In all cases, the returned
// response value will have the body drained and closed, so there is no need to close it again.
//
// If there are any network or I/O errors, the request will be retried as safely and resiliently as
// possible.
func (c *Client) httpReq(ctx context.Context, method, endpoint string, joseJSONPayload []byte, output interface{}) (*http.Response, error) {
	// even if the caller doesn't specify an output, we still use a
	// buffer to store possible error response (we reset it later)
	buf := bufPool.Get().(*bytes.Buffer)
	defer bufPool.Put(buf)

	var resp *http.Response
	var err error

	// potentially retry the request if there's network, I/O, or server internal errors
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// traffic calming ahead
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		var body io.Reader
		if joseJSONPayload != nil {
			body = bytes.NewReader(joseJSONPayload)
		}

		var req *http.Request
		req, err = http.NewRequestWithContext(ctx, method, endpoint, body)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		if len(joseJSONPayload) > 0 {
			req.Header.Set("Content-Type", "application/jose+json")
		}

		// on first attempt, we need to reset buf since it
		// came from the pool; after first attempt, we should
		// still reset it because we might be retrying after
		// a partial download
		buf.Reset()

		var retry bool
		resp, retry, err = c.doHTTPRequest(req, buf)
		if err != nil {
			if retry {
				if c.Logger != nil {
					c.Logger.Warn("HTTP request failed; retrying",
						zap.String("url", req.URL.String()),
						zap.Error(err))
				}
				continue
			}
			break
		}

		// check for HTTP errors
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300: // OK
		case resp.StatusCode >= 400 && resp.StatusCode < 600: // error
			if parseMediaType(resp) == "application/problem+json" {
				// "When the server responds with an error status, it SHOULD provide
				// additional information using a problem document [RFC7807]." (§6.7)
				var problem Problem
				err = json.Unmarshal(buf.Bytes(), &problem)
				if err != nil {
					return resp, fmt.Errorf("HTTP %d: JSON-decoding problem details: %w (raw='%s')",
						resp.StatusCode, err, buf.String())
				}
				if resp.StatusCode >= 500 && joseJSONPayload == nil {
					// a 5xx status is probably safe to retry on even after a
					// request that had no I/O errors; it could be that the
					// server just had a hiccup... so try again, but only if
					// there is no request body, because we can't replay a
					// request that has an anti-replay nonce, obviously
					err = problem
					continue
				}
				return resp, problem
			}
			return resp, fmt.Errorf("HTTP %d: %s", resp.StatusCode, buf.String())
		default: // what even is this
			return resp, fmt.Errorf("unexpected status code: HTTP %d", resp.StatusCode)
		}

		// do not retry if we got this far (success)
		break
	}
	if err != nil {
		return resp, err
	}

	// if expecting a body, finally decode it
	if output != nil {
		contentType := parseMediaType(resp)
		switch contentType {
		case "application/json":
			// unmarshal JSON
			err = json.Unmarshal(buf.Bytes(), output)
			if err != nil {
				return resp, fmt.Errorf("JSON-decoding response body: %w", err)
			}

		default:
			// don't interpret anything else here; just hope
			// it's a Writer and copy the bytes
			w, ok := output.(io.Writer)
			if !ok {
				return resp, fmt.Errorf("response Content-Type is %s but target container is not io.Writer: %T", contentType, output)
			}
			_, err = io.Copy(w, buf)
			if err != nil {
				return resp, err
			}
		}
	}

	return resp, nil
}

// doHTTPRequest performs an HTTP request at most one time. It returns the response
// (with drained and closed body), having drained any request body into buf. If
// retry == true is returned, then the request should be safe to retry in the case
// of an error. However, in some cases a retry may be recommended even if part of
// the response body has been read and written into buf. Thus, the buffer may have
// been partially written to and should be reset before being reused.
//
// This method remembers any nonce returned by the server.
func (c *Client) doHTTPRequest(req *http.Request, buf *bytes.Buffer) (resp *http.Response, retry bool, err error) {
	req.Header.Set("User-Agent", c.userAgent())

	resp, err = c.httpClient().Do(req)
	if err != nil {
		return resp, true, fmt.Errorf("performing request: %w", err)
	}
	defer resp.Body.Close()

	if c.Logger != nil {
		c.Logger.Debug("http request",
			zap.String("method", req.Method),
			zap.String("url", req.URL.String()),
			zap.Reflect("headers", req.Header),
			zap.Reflect("response_headers", resp.Header),
			zap.Int("status_code", resp.StatusCode))
	}

	// "The server MUST include a Replay-Nonce header field
	// in every successful response to a POST request and
	// SHOULD provide it in error responses as well." §6.5
	//
	// "Before sending a POST request to the server, an ACME
	// client needs to have a fresh anti-replay nonce to put
	// in the 'nonce' header of the JWS.  In most cases, the
	// client will have gotten a nonce from a previous
	// request." §7.2
	//
	// So basically, we need to remember the nonces we get
	// and use them at the next opportunity.
	c.nonces.push(resp.Header.Get(replayNonce))

	// drain the response body, even if we aren't keeping it
	// (this allows us to reuse the connection and also read
	// any error information)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		// this is likely a network or I/O error, but is it worth retrying?
		// technically the request has already completed, it was just our
		// download of the response that failed; so we probably should not
		// retry if the request succeeded... however, if there was an HTTP
		// error, it likely didn't count against any server-enforced rate
		// limits, and we DO want to know the error information, so it should
		// be safe to retry the request in those cases AS LONG AS there is
		// no request body, which in the context of ACME likely includes an
		// anti-replay nonce, which obviously we can't reuse
		retry = resp.StatusCode >= 400 && req.Body == nil
		return resp, retry, fmt.Errorf("reading response body: %w", err)
	}

	return resp, false, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient == nil {
		return http.DefaultClient
	}
	return c.HTTPClient
}

func (c *Client) userAgent() string {
	ua := fmt.Sprintf("acmez (%s; %s)", runtime.GOOS, runtime.GOARCH)
	if c.UserAgent != "" {
		ua = c.UserAgent + " " + ua
	}
	return ua
}

// extractLinks extracts the URL from the Link header with the
// designated relation rel. It may return more than value
// if there are multiple matching Link values.
//
// Originally by Isaac: https://github.com/eggsampler/acme
// and has been modified to support multiple matching Links.
func extractLinks(resp *http.Response, rel string) []string {
	if resp == nil {
		return nil
	}
	var links []string
	for _, l := range resp.Header["Link"] {
		matches := linkRegex.FindAllStringSubmatch(l, -1)
		for _, m := range matches {
			if len(m) != 3 {
				continue
			}
			if m[2] == rel {
				links = append(links, m[1])
			}
		}
	}
	return links
}

// parseMediaType returns only the media type from the
// Content-Type header of resp.
func parseMediaType(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	ct := resp.Header.Get("Content-Type")
	sep := strings.Index(ct, ";")
	if sep < 0 {
		return ct
	}
	return strings.TrimSpace(ct[:sep])
}

// retryAfter returns a duration from the response's Retry-After
// header field, if it exists. It can return an error if the
// header contains an invalid value. If there is no error but
// there is no Retry-After header provided, then the fallback
// duration is returned instead.
func retryAfter(resp *http.Response, fallback time.Duration) (time.Duration, error) {
	if resp == nil {
		return fallback, nil
	}
	raSeconds := resp.Header.Get("Retry-After")
	if raSeconds == "" {
		return fallback, nil
	}
	ra, err := strconv.Atoi(raSeconds)
	if err != nil || ra < 0 {
		return 0, fmt.Errorf("response had invalid Retry-After header: %s", raSeconds)
	}
	return time.Duration(ra) * time.Second, nil
}

var bufPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

var linkRegex = regexp.MustCompile(`<(.+?)>;\s*rel="(.+?)"`)
