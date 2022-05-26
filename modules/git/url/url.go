// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package url

import (
	"fmt"
	stdurl "net/url"
	"strings"
)

// ErrWrongURLFormat represents an error with wrong url format
type ErrWrongURLFormat struct {
	URL string
}

func (err ErrWrongURLFormat) Error() string {
	return fmt.Sprintf("git URL %s format is wrong", err.URL)
}

// GitURL represents a git URL
type GitURL struct {
	*stdurl.URL
	extraMark int // 0 no extra 1 scp 2 file path with no prefix
}

// String returns the URL's string
func (u *GitURL) String() string {
	switch u.extraMark {
	case 0:
		return u.URL.String()
	case 1:
		return fmt.Sprintf("%s@%s:%s", u.User.Username(), u.Host, u.Path)
	case 2:
		return u.Path
	default:
		return ""
	}
}

// Parse parse all kinds of git URL
func Parse(remote string) (*GitURL, error) {
	if strings.Contains(remote, "://") {
		u, err := stdurl.Parse(remote)
		if err != nil {
			return nil, err
		}
		return &GitURL{URL: u}, nil
	} else if strings.Contains(remote, "@") && strings.Contains(remote, ":") {
		url := stdurl.URL{
			Scheme: "ssh",
		}
		squareBrackets := false
		lastIndex := -1
	FOR:
		for i := 0; i < len(remote); i++ {
			switch remote[i] {
			case '@':
				url.User = stdurl.User(remote[:i])
				lastIndex = i + 1
			case ':':
				if !squareBrackets {
					url.Host = strings.ReplaceAll(remote[lastIndex:i], "%25", "%")
					if len(remote) <= i+1 {
						return nil, ErrWrongURLFormat{URL: remote}
					}
					url.Path = remote[i+1:]
					break FOR
				}
			case '[':
				squareBrackets = true
			case ']':
				squareBrackets = false
			}
		}
		return &GitURL{
			URL:       &url,
			extraMark: 1,
		}, nil
	}

	return &GitURL{
		URL: &stdurl.URL{
			Scheme: "file",
			Path:   remote,
		},
		extraMark: 2,
	}, nil
}
