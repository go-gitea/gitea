// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package lfs

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
)

// DetermineEndpoint determines an endpoint from the clone url or uses the specified LFS url.
func DetermineEndpoint(cloneurl, lfsurl string) *url.URL {
	if len(lfsurl) > 0 {
		return endpointFromURL(lfsurl)
	}
	return endpointFromCloneURL(cloneurl)
}

func endpointFromCloneURL(rawurl string) *url.URL {
	ep := endpointFromURL(rawurl)
	if ep == nil {
		return ep
	}

	ep.Path = strings.TrimSuffix(ep.Path, "/")

	if ep.Scheme == "file" {
		return ep
	}

	if path.Ext(ep.Path) == ".git" {
		ep.Path += "/info/lfs"
	} else {
		ep.Path += ".git/info/lfs"
	}

	return ep
}

func endpointFromURL(rawurl string) *url.URL {
	if strings.HasPrefix(rawurl, "/") {
		return endpointFromLocalPath(rawurl)
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		log.Error("lfs.endpointFromUrl: %v", err)
		return nil
	}

	switch u.Scheme {
	case "http", "https":
		return u
	case "git":
		u.Scheme = "https"
		return u
	case "file":
		return u
	default:
		if _, err := os.Stat(rawurl); err == nil {
			return endpointFromLocalPath(rawurl)
		}

		log.Error("lfs.endpointFromUrl: unknown url")
		return nil
	}
}

func endpointFromLocalPath(path string) *url.URL {
	var slash string
	if abs, err := filepath.Abs(path); err == nil {
		if !strings.HasPrefix(abs, "/") {
			slash = "/"
		}
		path = abs
	}

	var gitpath string
	if filepath.Base(path) == ".git" {
		gitpath = path
		path = filepath.Dir(path)
	} else {
		gitpath = filepath.Join(path, ".git")
	}

	if _, err := os.Stat(gitpath); err == nil {
		path = gitpath
	} else if _, err := os.Stat(path); err != nil {
		return nil
	}

	path = fmt.Sprintf("file://%s%s", slash, filepath.ToSlash(path))

	u, _ := url.Parse(path)

	return u
}
