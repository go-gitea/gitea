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

	"code.gitea.io/gitea/modules/util"
)

// FilesystemClient is used to read LFS data from a filesystem path
type FilesystemClient struct {
	lfsdir string
}

func newFilesystemClient(endpoint *url.URL) *FilesystemClient {
	path, _ := util.FileURLToPath(endpoint)

	lfsdir := filepath.Join(path, "lfs", "objects")

	client := &FilesystemClient{lfsdir}

	return client
}

func (c *FilesystemClient) objectPath(oid string) string {
	return filepath.Join(c.lfsdir, oid[0:2], oid[2:4], oid)
}

// Download reads the specific LFS object from the target path
func (c *FilesystemClient) Download(ctx context.Context, p Pointer) (io.ReadCloser, error) {
	objectPath := c.objectPath(p.Oid)

	return os.Open(objectPath)
}

// Upload writes the specific LFS object to the target path
func (c *FilesystemClient) Upload(ctx context.Context, p Pointer, r io.Reader) error {
	objectPath := c.objectPath(p.Oid)

	if err := os.MkdirAll(filepath.Dir(objectPath), 0600); err != nil {
		return err
	}

	f, err := os.Create(objectPath)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, r)

	return err
}
