// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package url

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// URL represents a git remote URL
type URL struct {
	*url.URL
	extraMark int // 0 no extra 1 scp 2 file path with no prefix
}

// String returns the URL's string
func (u *URL) String() string {
	switch u.extraMark {
	case 0:
		return u.String()
	case 1:
		return fmt.Sprintf("%s@%s:%s", u.User.Username(), u.Host, u.Path)
	case 2:
		return u.Path
	default:
		return ""
	}
}

var scpSyntaxRe = regexp.MustCompile(`^([a-zA-Z0-9_]+)@([a-zA-Z0-9._-]+):(.*)$`)

// Parse parse all kinds of git remote URL
func Parse(remote string) (*URL, error) {
	u, err := url.Parse(remote)
	if err == nil {
		extraMark := 0
		if u.Scheme == "" && u.Path != "" {
			u.Scheme = "file"
			extraMark = 2
		}
		return &URL{URL: u, extraMark: extraMark}, nil
	}

	if !strings.Contains(err.Error(), "first path segment in URL cannot contain colon") {
		return nil, err
	}

	if results := scpSyntaxRe.FindStringSubmatch(remote); results != nil {
		return &URL{
			URL: &url.URL{
				Scheme: "ssh",
				User:   url.User(results[1]),
				Host:   results[2],
				Path:   results[3],
			},
			extraMark: 1,
		}, nil
	}

	return &URL{
		URL: &url.URL{
			Scheme: "file",
			Path:   remote,
		},
		extraMark: 2,
	}, nil
}
