// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

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
	u, err := url.Parse(uriStr)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		f, err := http.Get(uriStr)
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
