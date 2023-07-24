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

// NewMinioStorage returns a minio storage
func NewMinioStorage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	config := cfg.MinioConfig
	if config.ChecksumAlgorithm != "" && config.ChecksumAlgorithm != "default" && config.ChecksumAlgorithm != "md5" {
		return nil, fmt.Errorf("invalid minio checksum algorithm: %s", config.ChecksumAlgorithm)
	}

	log.Info("Creating Minio storage at %s:%s with base path %s", config.Endpoint, config.Bucket, config.BasePath)

	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure:    config.UseSSL,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}},
	})
	if err != nil {
		return nil, convertMinioErr(err)
	}

	if err := minioClient.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{
		Region: config.Location,
	}); err != nil {
		// Check to see if we already own this bucket (which happens if you run this twice)
		exists, errBucketExists := minioClient.BucketExists(ctx, config.Bucket)
		if !exists || errBucketExists != nil {
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
	p = util.PathJoinRelX(m.basePath, p)
	if p == "." {
		p = "" // minio doesn't use dot as relative path
	}
	return p
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
	lobjectCtx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	basePath := m.basePath
	if dirName != "" {
		// ending slash is required for avoiding matching like "foo/" and "foobar/" with prefix "foo"
		basePath = m.buildMinioPath(dirName) + "/"
	}

	for mObjInfo := range m.client.ListObjects(lobjectCtx, m.bucket, minio.ListObjectsOptions{
		Prefix:    basePath,
		Recursive: true,
	}) {
		object, err := m.client.GetObject(lobjectCtx, m.bucket, mObjInfo.Key, opts)
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
