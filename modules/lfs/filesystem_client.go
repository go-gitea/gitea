// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

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
	lfsDir string
}

// BatchSize returns the preferred size of batchs to process
func (c *FilesystemClient) BatchSize() int {
	return 1
}

func newFilesystemClient(endpoint *url.URL) *FilesystemClient {
	path, _ := util.FileURLToPath(endpoint)
	lfsDir := filepath.Join(path, "lfs", "objects")
	return &FilesystemClient{lfsDir}
}

func (c *FilesystemClient) objectPath(oid string) string {
	return filepath.Join(c.lfsDir, oid[0:2], oid[2:4], oid)
}

// Download reads the specific LFS object from the target path
func (c *FilesystemClient) Download(ctx context.Context, objects []Pointer, callback DownloadCallback) error {
	for _, object := range objects {
		p := Pointer{object.Oid, object.Size}

		objectPath := c.objectPath(p.Oid)

		f, err := os.Open(objectPath)
		if err != nil {
			return err
		}

		if err := callback(p, f, nil); err != nil {
			return err
		}
	}
	return nil
}

// Upload writes the specific LFS object to the target path
func (c *FilesystemClient) Upload(ctx context.Context, objects []Pointer, callback UploadCallback) error {
	for _, object := range objects {
		p := Pointer{object.Oid, object.Size}

		objectPath := c.objectPath(p.Oid)

		if err := os.MkdirAll(filepath.Dir(objectPath), os.ModePerm); err != nil {
			return err
		}

		content, err := callback(p, nil)
		if err != nil {
			return err
		}

		err = func() error {
			defer content.Close()

			f, err := os.Create(objectPath)
			if err != nil {
				return err
			}

			_, err = io.Copy(f, content)

			return err
		}()
		if err != nil {
			return err
		}
	}
	return nil
}
