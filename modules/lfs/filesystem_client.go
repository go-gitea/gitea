// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lfs

import (
	"context"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
)

// FilesystemClient is used to read LFS data from a filesystem path
type FilesystemClient struct {
	lfsdir string
}

func newFilesystemClient(endpoint *url.URL) *FilesystemClient {
	lfsdir := filepath.Join(endpointURLToPath(endpoint), "lfs", "objects")

	client := &FilesystemClient{lfsdir}

	return client
}

func endpointURLToPath(endpoint *url.URL) string {
	path := endpoint.Path

	if runtime.GOOS != "windows" {
		return path
	}

	// If it looks like there's a Windows drive letter at the beginning, strip off the leading slash.
	re := regexp.MustCompile("/[A-Za-z]:/")
	if re.MatchString(path) {
		return path[1:]
	}
	return path
}

func (c *FilesystemClient) objectPath(oid string) string {
	return filepath.Join(c.lfsdir, oid[0:2], oid[2:4], oid)
}

// Download reads the specific LFS object from the target repository
func (c *FilesystemClient) Download(ctx context.Context, oid string, size int64) (io.ReadCloser, error) {
	objectPath := c.objectPath(oid)

	if _, err := os.Stat(objectPath); os.IsNotExist(err) {
		return nil, err
	}

	file, err := os.Open(objectPath)
	if err != nil {
		return nil, err
	}

	return file, nil
}
