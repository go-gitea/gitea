// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uri

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// ErrURISchemeNotSupported represents a scheme error
type ErrURISchemeNotSupported struct {
	Scheme string
}

func (e ErrURISchemeNotSupported) Error() string {
	return fmt.Sprintf("Unsupported scheme: %v", e.Scheme)
}

// Open open a local file or a remote file
func Open(uriStr string) (io.ReadCloser, error) {
	return OpenWithClient(uriStr, http.DefaultClient)
}

// OpenWithClient opens a local file or a remote file, using the given (non-nil) HTTP client
// for http/https URLs. Callers that must confine remote access (e.g. to defeat SSRF via
// redirects) should pass a client whose transport validates the peer at dial time; Open
// passes http.DefaultClient.
func OpenWithClient(uriStr string, client *http.Client) (io.ReadCloser, error) {
	u, err := url.Parse(uriStr)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		f, err := client.Get(uriStr)
		if err != nil {
			return nil, err
		}
		return f.Body, nil
	case "file":
		return os.Open(u.Path)
	default:
		return nil, ErrURISchemeNotSupported{Scheme: u.Scheme}
	}
}
