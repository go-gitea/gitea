// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	_            ObjectStorage = &MinioStorage{}
	quoteEscaper               = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
)

// MinioStorage returns a minio bucket storage
type MinioStorage struct {
	ctx      context.Context
	client   *minio.Client
	bucket   string
	basePath string
}

// NewMinioStorage returns a minio storage
func NewMinioStorage(ctx context.Context, endpoint, accessKeyID, secretAccessKey, bucket, location, basePath string, useSSL bool) (*MinioStorage, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}

	if err := minioClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{
		Region: location,
	}); err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(ctx, bucket)
		if !exists || errBucketExists != nil {
			return nil, err
		}
	}

	return &MinioStorage{
		ctx:      ctx,
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
	object, err := m.client.GetObject(m.ctx, m.bucket, m.buildMinioPath(path), opts)
	if err != nil {
		return nil, err
	}
	return object, nil
}

// Save save a file to minio
func (m *MinioStorage) Save(path string, r io.Reader) (int64, error) {
	uploadInfo, err := m.client.PutObject(
		m.ctx,
		m.bucket,
		m.buildMinioPath(path),
		r,
		-1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"},
	)
	if err != nil {
		return 0, err
	}
	return uploadInfo.Size, nil
}

// Delete delete a file
func (m *MinioStorage) Delete(path string) error {
	return m.client.RemoveObject(m.ctx, m.bucket, m.buildMinioPath(path), minio.RemoveObjectOptions{})
}

// URL gets the redirect URL to a file. The presigned link is valid for 5 minutes.
func (m *MinioStorage) URL(path, name string) (*url.URL, error) {
	reqParams := make(url.Values)
	// TODO it may be good to embed images with 'inline' like ServeData does, but we don't want to have to read the file, do we?
	reqParams.Set("response-content-disposition", "attachment; filename=\""+quoteEscaper.Replace(name)+"\"")
	return m.client.PresignedGetObject(m.ctx, m.bucket, m.buildMinioPath(path), 5*time.Minute, reqParams)
}
