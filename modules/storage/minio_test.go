// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	awshttp "github.com/aws/smithy-go/transport/http"
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
	assert.Empty(t, m.buildMinioPath("/"))
	assert.Empty(t, m.buildMinioPath("."))
	assert.Equal(t, "a", m.buildMinioPath("/a"))
	assert.Equal(t, "a/b", m.buildMinioPath("/a/b/"))
	assert.Empty(t, m.buildMinioDirPrefix(""))
	assert.Equal(t, "a/", m.buildMinioDirPrefix("/a/"))

	m = &MinioStorage{basePath: "/"}
	assert.Empty(t, m.buildMinioPath("/"))
	assert.Empty(t, m.buildMinioPath("."))
	assert.Equal(t, "a", m.buildMinioPath("/a"))
	assert.Equal(t, "a/b", m.buildMinioPath("/a/b/"))
	assert.Empty(t, m.buildMinioDirPrefix(""))
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
	getBucketVersioning = func(ctx context.Context, client *s3.Client, bucket string) error {
		return &awshttp.ResponseError{
			Response: &awshttp.Response{
				Response: &http.Response{
					StatusCode: http.StatusBadRequest,
				},
			},
			Err: errors.New(message),
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
		credProvider := buildS3CredentialsProvider(cfg)
		creds, err := credProvider.Retrieve(context.Background())

		assert.NoError(t, err)
		assert.Equal(t, ExpectedAccessKey, creds.AccessKeyID)
		assert.Equal(t, ExpectedSecretAccessKey, creds.SecretAccessKey)
	})

	t.Run("Chain", func(t *testing.T) {
		cfg := setting.MinioStorageConfig{
			IamEndpoint: FakeEndpoint,
		}

		t.Run("EnvMinio", func(t *testing.T) {
			t.Setenv("MINIO_ACCESS_KEY", ExpectedAccessKey+"Minio")
			t.Setenv("MINIO_SECRET_KEY", ExpectedSecretAccessKey+"Minio")

			credProvider := buildS3CredentialsProvider(cfg)
			creds, err := credProvider.Retrieve(context.Background())

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"Minio", creds.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"Minio", creds.SecretAccessKey)
		})

		t.Run("EnvAWS", func(t *testing.T) {
			t.Setenv("AWS_ACCESS_KEY", ExpectedAccessKey+"AWS")
			t.Setenv("AWS_SECRET_KEY", ExpectedSecretAccessKey+"AWS")

			credProvider := buildS3CredentialsProvider(cfg)
			creds, err := credProvider.Retrieve(context.Background())

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"AWS", creds.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"AWS", creds.SecretAccessKey)
		})

		t.Run("FileMinio", func(t *testing.T) {
			// prevent loading any actual credentials files from the user
			t.Setenv("MINIO_SHARED_CREDENTIALS_FILE", "testdata/minio.json")
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "testdata/fake")

			credProvider := buildS3CredentialsProvider(cfg)
			creds, err := credProvider.Retrieve(context.Background())

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"MinioFile", creds.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"MinioFile", creds.SecretAccessKey)
		})

		t.Run("FileAWS", func(t *testing.T) {
			// prevent loading any actual credentials files from the user
			t.Setenv("MINIO_SHARED_CREDENTIALS_FILE", "testdata/fake.json")
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "testdata/aws_credentials")

			credProvider := buildS3CredentialsProvider(cfg)
			creds, err := credProvider.Retrieve(context.Background())

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"AWSFile", creds.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"AWSFile", creds.SecretAccessKey)
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
			credProvider := buildS3CredentialsProvider(setting.MinioStorageConfig{
				IamEndpoint: server.URL,
			})
			creds, err := credProvider.Retrieve(context.Background())

			assert.NoError(t, err)
			assert.Equal(t, ExpectedAccessKey+"IAM", creds.AccessKeyID)
			assert.Equal(t, ExpectedSecretAccessKey+"IAM", creds.SecretAccessKey)
		})
	})
}
