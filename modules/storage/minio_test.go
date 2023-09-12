// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"net/http"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
)

func TestMinioStorageIterator(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("minioStorage not present outside of CI")
		return
	}
	testStorageIterator(t, setting.MinioStorageType, &setting.Storage{
		MinioConfig: setting.MinioStorageConfig{
			Endpoint:        "127.0.0.1:9000",
			AccessKeyID:     "123456",
			SecretAccessKey: "12345678",
			Bucket:          "gitea",
			Location:        "us-east-1",
		},
	})
}

func TestS3StorageBadRequest(t *testing.T) {
	if os.Getenv("CI") == "" {
		t.Skip("S3Storage not present outside of CI")
		return
	}
	cfg := &setting.Storage{
		MinioConfig: setting.MinioStorageConfig{
			Endpoint:        "minio:9000",
			AccessKeyID:     "123456",
			SecretAccessKey: "12345678",
			Bucket:          "bucket",
			Location:        "us-east-1",
		},
	}
	message := "ERROR"
	old := getBucketVersioning
	defer func() { getBucketVersioning = old }()
	getBucketVersioning = func(ctx context.Context, minioClient *minio.Client, bucket string) error {
		return minio.ErrorResponse{
			StatusCode: http.StatusBadRequest,
			Code:       "FixtureError",
			Message:    message,
		}
	}
	_, err := NewStorage(setting.MinioStorageType, cfg)
	assert.ErrorContains(t, err, message)
}
