// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
)

var (
	// ErrResponseSize is raised if a response body exceeds the given max body size.
	ErrResponseSize = errors.New("elastic: response size too large")
)

// Response represents a response from Elasticsearch.
type Response struct {
	// StatusCode is the HTTP status code, e.g. 200.
	StatusCode int
	// Header is the HTTP header from the HTTP response.
	// Keys in the map are canonicalized (see http.CanonicalHeaderKey).
	Header http.Header
	// Body is the deserialized response body.
	Body json.RawMessage
	// DeprecationWarnings lists all deprecation warnings returned from
	// Elasticsearch.
	DeprecationWarnings []string
}

// newResponse creates a new response from the HTTP response.
func (c *Client) newResponse(res *http.Response, maxBodySize int64) (*Response, error) {
	r := &Response{
		StatusCode:          res.StatusCode,
		Header:              res.Header,
		DeprecationWarnings: res.Header["Warning"],
	}
	if res.Body != nil {
		body := io.Reader(res.Body)
		if maxBodySize > 0 {
			if res.ContentLength > maxBodySize {
				return nil, ErrResponseSize
			}
			body = io.LimitReader(body, maxBodySize+1)
		}
		slurp, err := ioutil.ReadAll(body)
		if err != nil {
			return nil, err
		}
		if maxBodySize > 0 && int64(len(slurp)) > maxBodySize {
			return nil, ErrResponseSize
		}
		// HEAD requests return a body but no content
		if len(slurp) > 0 {
			r.Body = json.RawMessage(slurp)
		}
	}
	return r, nil
}
