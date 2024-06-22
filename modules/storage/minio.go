// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	_ ObjectStorage = &MinioStorage{}

	quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")
)

type minioObject struct {
	*minio.Object
}

func (m *minioObject) Stat() (os.FileInfo, error) {
	oi, err := m.Object.Stat()
	if err != nil {
		return nil, convertMinioErr(err)
	}

	return &minioFileInfo{oi}, nil
}

// MinioStorage returns a minio bucket storage
type MinioStorage struct {
	cfg      *setting.MinioStorageConfig
	ctx      context.Context
	client   *minio.Client
	bucket   string
	basePath string
}

func convertMinioErr(err error) error {
	if err == nil {
		return nil
	}
	errResp, ok := err.(minio.ErrorResponse)
	if !ok {
		return err
	}

	// Convert two responses to standard analogues
	switch errResp.Code {
	case "NoSuchKey":
		return os.ErrNotExist
	case "AccessDenied":
		return os.ErrPermission
	}

	return err
}

var getBucketVersioning = func(ctx context.Context, minioClient *minio.Client, bucket string) error {
	_, err := minioClient.GetBucketVersioning(ctx, bucket)
	return err
}

// NewMinioStorage returns a minio storage
func NewMinioStorage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	config := cfg.MinioConfig
	if config.ChecksumAlgorithm != "" && config.ChecksumAlgorithm != "default" && config.ChecksumAlgorithm != "md5" {
		return nil, fmt.Errorf("invalid minio checksum algorithm: %s", config.ChecksumAlgorithm)
	}

	log.Info("Creating Minio storage at %s:%s with base path %s", config.Endpoint, config.Bucket, config.BasePath)

	var lookup minio.BucketLookupType
	if config.BucketLookUpType == "auto" || config.BucketLookUpType == "" {
		lookup = minio.BucketLookupAuto
	} else if config.BucketLookUpType == "dns" {
		lookup = minio.BucketLookupDNS
	} else if config.BucketLookUpType == "path" {
		lookup = minio.BucketLookupPath
	} else {
		return nil, fmt.Errorf("invalid minio bucket lookup type: %s", config.BucketLookUpType)
	}

	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:        buildMinioCredentials(config, credentials.DefaultIAMRoleEndpoint),
		Secure:       config.UseSSL,
		Transport:    &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}},
		Region:       config.Location,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, convertMinioErr(err)
	}

	// The GetBucketVersioning is only used for checking whether the Object Storage parameters are generally good. It doesn't need to succeed.
	// The assumption is that if the API returns the HTTP code 400, then the parameters could be incorrect.
	// Otherwise even if the request itself fails (403, 404, etc), the code should still continue because the parameters seem "good" enough.
	// Keep in mind that GetBucketVersioning requires "owner" to really succeed, so it can't be used to check the existence.
	// Not using "BucketExists (HeadBucket)" because it doesn't include detailed failure reasons.
	err = getBucketVersioning(ctx, minioClient, config.Bucket)
	if err != nil {
		errResp, ok := err.(minio.ErrorResponse)
		if !ok {
			return nil, err
		}
		if errResp.StatusCode == http.StatusBadRequest {
			log.Error("S3 storage connection failure at %s:%s with base path %s and region: %s", config.Endpoint, config.Bucket, config.Location, errResp.Message)
			return nil, err
		}
	}

	// Check to see if we already own this bucket
	exists, err := minioClient.BucketExists(ctx, config.Bucket)
	if err != nil {
		return nil, convertMinioErr(err)
	}

	if !exists {
		if err := minioClient.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{
			Region: config.Location,
		}); err != nil {
			return nil, convertMinioErr(err)
		}
	}

	return &MinioStorage{
		cfg:      &config,
		ctx:      ctx,
		client:   minioClient,
		bucket:   config.Bucket,
		basePath: config.BasePath,
	}, nil
}

func (m *MinioStorage) buildMinioPath(p string) string {
	p = strings.TrimPrefix(util.PathJoinRelX(m.basePath, p), "/") // object store doesn't use slash for root path
	if p == "." {
		p = "" // object store doesn't use dot as relative path
	}
	return p
}

func (m *MinioStorage) buildMinioDirPrefix(p string) string {
	// ending slash is required for avoiding matching like "foo/" and "foobar/" with prefix "foo"
	p = m.buildMinioPath(p) + "/"
	if p == "/" {
		p = "" // object store doesn't use slash for root path
	}
	return p
}

func buildMinioCredentials(config setting.MinioStorageConfig, iamEndpoint string) *credentials.Credentials {
	// If static credentials are provided, use those
	if config.AccessKeyID != "" {
		return credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, "")
	}

	// Otherwise, fallback to a credentials chain for S3 access
	chain := []credentials.Provider{
		// configure based upon MINIO_ prefixed environment variables
		&credentials.EnvMinio{},
		// configure based upon AWS_ prefixed environment variables
		&credentials.EnvAWS{},
		// read credentials from MINIO_SHARED_CREDENTIALS_FILE
		// environment variable, or default json config files
		&credentials.FileMinioClient{},
		// read credentials from AWS_SHARED_CREDENTIALS_FILE
		// environment variable, or default credentials file
		&credentials.FileAWSCredentials{},
		// read IAM role from EC2 metadata endpoint if available
		&credentials.IAM{
			Endpoint: iamEndpoint,
			Client: &http.Client{
				Transport: http.DefaultTransport,
			},
		},
	}
	return credentials.NewChainCredentials(chain)
}

// Open opens a file
func (m *MinioStorage) Open(path string) (Object, error) {
	opts := minio.GetObjectOptions{}
	object, err := m.client.GetObject(m.ctx, m.bucket, m.buildMinioPath(path), opts)
	if err != nil {
		return nil, convertMinioErr(err)
	}
	return &minioObject{object}, nil
}

// Save saves a file to minio
func (m *MinioStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	uploadInfo, err := m.client.PutObject(
		m.ctx,
		m.bucket,
		m.buildMinioPath(path),
		r,
		size,
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
			// some storages like:
			// * https://developers.cloudflare.com/r2/api/s3/api/
			// * https://www.backblaze.com/b2/docs/s3_compatible_api.html
			// do not support "x-amz-checksum-algorithm" header, so use legacy MD5 checksum
			SendContentMd5: m.cfg.ChecksumAlgorithm == "md5",
		},
	)
	if err != nil {
		return 0, convertMinioErr(err)
	}
	return uploadInfo.Size, nil
}

type minioFileInfo struct {
	minio.ObjectInfo
}

func (m minioFileInfo) Name() string {
	return path.Base(m.ObjectInfo.Key)
}

func (m minioFileInfo) Size() int64 {
	return m.ObjectInfo.Size
}

func (m minioFileInfo) ModTime() time.Time {
	return m.LastModified
}

func (m minioFileInfo) IsDir() bool {
	return strings.HasSuffix(m.ObjectInfo.Key, "/")
}

func (m minioFileInfo) Mode() os.FileMode {
	return os.ModePerm
}

func (m minioFileInfo) Sys() any {
	return nil
}

// Stat returns the stat information of the object
func (m *MinioStorage) Stat(path string) (os.FileInfo, error) {
	info, err := m.client.StatObject(
		m.ctx,
		m.bucket,
		m.buildMinioPath(path),
		minio.StatObjectOptions{},
	)
	if err != nil {
		return nil, convertMinioErr(err)
	}
	return &minioFileInfo{info}, nil
}

// Delete delete a file
func (m *MinioStorage) Delete(path string) error {
	err := m.client.RemoveObject(m.ctx, m.bucket, m.buildMinioPath(path), minio.RemoveObjectOptions{})

	return convertMinioErr(err)
}

// URL gets the redirect URL to a file. The presigned link is valid for 5 minutes.
func (m *MinioStorage) URL(path, name string) (*url.URL, error) {
	reqParams := make(url.Values)
	// TODO it may be good to embed images with 'inline' like ServeData does, but we don't want to have to read the file, do we?
	reqParams.Set("response-content-disposition", "attachment; filename=\""+quoteEscaper.Replace(name)+"\"")
	u, err := m.client.PresignedGetObject(m.ctx, m.bucket, m.buildMinioPath(path), 5*time.Minute, reqParams)
	return u, convertMinioErr(err)
}

// IterateObjects iterates across the objects in the miniostorage
func (m *MinioStorage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	opts := minio.GetObjectOptions{}
	for mObjInfo := range m.client.ListObjects(m.ctx, m.bucket, minio.ListObjectsOptions{
		Prefix:    m.buildMinioDirPrefix(dirName),
		Recursive: true,
	}) {
		object, err := m.client.GetObject(m.ctx, m.bucket, mObjInfo.Key, opts)
		if err != nil {
			return convertMinioErr(err)
		}
		if err := func(object *minio.Object, fn func(path string, obj Object) error) error {
			defer object.Close()
			return fn(strings.TrimPrefix(mObjInfo.Key, m.basePath), &minioObject{object})
		}(object, fn); err != nil {
			return convertMinioErr(err)
		}
	}
	return nil
}

func init() {
	RegisterStorageType(setting.MinioStorageType, NewMinioStorage)
}
