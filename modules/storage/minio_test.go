// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
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
			Endpoint:        "minio:9000",
			AccessKeyID:     "123456",
			SecretAccessKey: "12345678",
			Bucket:          "gitea",
			Location:        "us-east-1",
		},
	})
}

func TestMinioStoragePath(t *testing.T) {
	m := &MinioStorage{basePath: ""}
	assert.Equal(t, "", m.buildMinioPath("/"))
	assert.Equal(t, "", m.buildMinioPath("."))
	assert.Equal(t, "a", m.buildMinioPath("/a"))
	assert.Equal(t, "a/b", m.buildMinioPath("/a/b/"))
	assert.Equal(t, "", m.buildMinioDirPrefix(""))
	assert.Equal(t, "a/", m.buildMinioDirPrefix("/a/"))

	m = &MinioStorage{basePath: "/"}
	assert.Equal(t, "", m.buildMinioPath("/"))
	assert.Equal(t, "", m.buildMinioPath("."))
	assert.Equal(t, "a", m.buildMinioPath("/a"))
	assert.Equal(t, "a/b", m.buildMinioPath("/a/b/"))
	assert.Equal(t, "", m.buildMinioDirPrefix(""))
	assert.Equal(t, "a/", m.buildMinioDirPrefix("/a/"))

	m = &MinioStorage{basePath: "/base"}
	assert.Equal(t, "base", m.buildMinioPath("/"))
	assert.Equal(t, "base", m.buildMinioPath("."))
	assert.Equal(t, "base/a", m.buildMinioPath("/a"))
	assert.Equal(t, "base/a/b", m.buildMinioPath("/a/b/"))
	assert.Equal(t, "base/", m.buildMinioDirPrefix(""))
	assert.Equal(t, "base/a/", m.buildMinioDirPrefix("/a/"))

	m = &MinioStorage{basePath: "/base/"}
	assert.Equal(t, "base", m.buildMinioPath("/"))
	assert.Equal(t, "base", m.buildMinioPath("."))
	assert.Equal(t, "base/a", m.buildMinioPath("/a"))
	assert.Equal(t, "base/a/b", m.buildMinioPath("/a/b/"))
	assert.Equal(t, "base/", m.buildMinioDirPrefix(""))
	assert.Equal(t, "base/a/", m.buildMinioDirPrefix("/a/"))
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

func TestMinioCredentials(t *testing.T) {
	const (
		ExpectedAccessKey       = "ExampleAccessKeyID"
		ExpectedSecretAccessKey = "ExampleSecretAccessKeyID"
		// Use a FakeEndpoint for IAM credentials to avoid logging any
		// potential real IAM credentials when running in EC2.
		FakeEndpoint = "http://localhost"
	)

	t.Run("Static Credentials", func(t *testing.T) {
		cfg := setting.MinioStorageConfig{
			AccessKeyID:     ExpectedAccessKey,
			SecretAccessKey: ExpectedSecretAccessKey,
			IamEndpoint:     FakeEndpoint,
		}
		creds := buildMinioCredentials(cfg)
		v, err := creds.Get()

		assert.NoError(t, err)
		assert.Equal(t, ExpectedAccessKey, v.AccessKeyID)
		assert.Equal(t, ExpectedSecretAccessKey, v.SecretAccessKey)
	})

	t.Run("Chain", func(t *testing.T) {
		cfg := setting.MinioStorageConfig{
			IamEndpoint: FakeEndpoint,
		}

		t.Run("EnvMinio", func(t *testing.T) {
			t.Setenv("MINIO_ACCESS_KEY", ExpectedAccessKey+"Minio")
			t.Setenv("MINIO_SECRET_KEY", ExpectedSecretAccessKey+"Minio")

			creds := buildMinioCredentials(cfg)
			v, err := creds.Get()

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"Minio", v.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"Minio", v.SecretAccessKey)
		})

		t.Run("EnvAWS", func(t *testing.T) {
			t.Setenv("AWS_ACCESS_KEY", ExpectedAccessKey+"AWS")
			t.Setenv("AWS_SECRET_KEY", ExpectedSecretAccessKey+"AWS")

			creds := buildMinioCredentials(cfg)
			v, err := creds.Get()

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"AWS", v.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"AWS", v.SecretAccessKey)
		})

		t.Run("FileMinio", func(t *testing.T) {
			// prevent loading any actual credentials files from the user
			t.Setenv("MINIO_SHARED_CREDENTIALS_FILE", "testdata/minio.json")
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "testdata/fake")

			creds := buildMinioCredentials(cfg)
			v, err := creds.Get()

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"MinioFile", v.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"MinioFile", v.SecretAccessKey)
		})

		t.Run("FileAWS", func(t *testing.T) {
			// prevent loading any actual credentials files from the user
			t.Setenv("MINIO_SHARED_CREDENTIALS_FILE", "testdata/fake.json")
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "testdata/aws_credentials")

			creds := buildMinioCredentials(cfg)
			v, err := creds.Get()

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"AWSFile", v.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"AWSFile", v.SecretAccessKey)
		})

		t.Run("IAM", func(t *testing.T) {
			// prevent loading any actual credentials files from the user
			t.Setenv("MINIO_SHARED_CREDENTIALS_FILE", "testdata/fake.json")
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "testdata/fake")

			// Spawn a server to emulate the EC2 Instance Metadata
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// The client will actually make 3 requests here,
				// first will be to get the IMDSv2 token, second to
				// get the role, and third for the actual
				// credentials. However, we can return credentials
				// every request since we're not emulating a full
				// IMDSv2 flow.
				w.Write([]byte(`{"Code":"Success","AccessKeyId":"ExampleAccessKeyIDIAM","SecretAccessKey":"ExampleSecretAccessKeyIDIAM"}`))
			}))
			defer server.Close()

			// Use the provided EC2 Instance Metadata server
			creds := buildMinioCredentials(setting.MinioStorageConfig{
				IamEndpoint: server.URL,
			})
			v, err := creds.Get()

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"IAM", v.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"IAM", v.SecretAccessKey)
		})
	})
}
