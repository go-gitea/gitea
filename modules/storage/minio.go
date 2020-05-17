// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"io"
	"path"
	"strings"

	"github.com/minio/minio-go"
)

var (
	_ ObjectStorage = &MinioStorage{}
)

// MinioStorage returns a minio bucket storage
type MinioStorage struct {
	client   *minio.Client
	bucket   string
	location string
	basePath string
}

// NewMinioStorage returns a minio storage
func NewMinioStorage(endpoint, accessKeyID, secretAccessKey, bucket, location, basePath string, useSSL bool) (*MinioStorage, error) {
	minioClient, err := minio.New(endpoint, accessKeyID, secretAccessKey, useSSL)
	if err != nil {
		return nil, err
	}

	if err := minioClient.MakeBucket(bucket, location); err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(bucket)
		if !exists || errBucketExists != nil {
			return nil, err
		}
	}

	return &MinioStorage{
		client:   minioClient,
		bucket:   bucket,
		basePath: basePath,
	}, nil
}

func (m *MinioStorage) buildMinioPath(p string) string {
	return strings.TrimPrefix(path.Join(m.basePath, p), "/")
}

// Open open a file
func (m *MinioStorage) Open(path string) (io.ReadCloser, error) {
	var opts = minio.GetObjectOptions{}
	object, err := m.client.GetObject(m.bucket, m.buildMinioPath(path), opts)
	if err != nil {
		return nil, err
	}
	return object, nil
}

// Save save a file to minio
func (m *MinioStorage) Save(path string, r io.Reader) (int64, error) {
	return m.client.PutObject(m.bucket, m.buildMinioPath(path), r, -1, minio.PutObjectOptions{ContentType: "application/octet-stream"})
}

// Delete delete a file
func (m *MinioStorage) Delete(path string) error {
	return m.client.RemoveObject(m.bucket, m.buildMinioPath(path))
}
