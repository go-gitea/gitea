// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
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

// MinioStorageType is the type descriptor for minio storage
const MinioStorageType Type = "minio"

// MinioStorageConfig represents the configuration for a minio storage
type MinioStorageConfig struct {
	Endpoint           string `ini:"MINIO_ENDPOINT"`
	AccessKeyID        string `ini:"MINIO_ACCESS_KEY_ID"`
	SecretAccessKey    string `ini:"MINIO_SECRET_ACCESS_KEY"`
	Bucket             string `ini:"MINIO_BUCKET"`
	Location           string `ini:"MINIO_LOCATION"`
	BasePath           string `ini:"MINIO_BASE_PATH"`
	UseSSL             bool   `ini:"MINIO_USE_SSL"`
  InsecureSkipVerify bool   `ini:"MINIO_INSECURE_SKIP_VERIFY"`
	DisableSignature   bool   `ini:"MINIO_DISABLE_SIGNATURE"`
	DisableMultipart   bool   `ini:"MINIO_DISABLE_MULTIPART"`
}

// MinioStorage returns a minio bucket storage
type MinioStorage struct {
	ctx      context.Context
	client   *minio.Client
	bucket   string
	basePath string
	config   *MinioStorageConfig
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
func NewMinioStorage(ctx context.Context, cfg interface{}) (ObjectStorage, error) {
	configInterface, err := toConfig(MinioStorageConfig{}, cfg)
	if err != nil {
		return nil, convertMinioErr(err)
	}
	config := configInterface.(MinioStorageConfig)

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
		ctx:      ctx,
		client:   minioClient,
		bucket:   config.Bucket,
		basePath: config.BasePath,
		config:   &config,
	}, nil
}

func (m *MinioStorage) buildMinioPath(p string) string {
	return util.PathJoinRelX(m.basePath, p)
}

// Open open a file
func (m *MinioStorage) Open(path string) (Object, error) {
	opts := minio.GetObjectOptions{}
	object, err := m.client.GetObject(m.ctx, m.bucket, m.buildMinioPath(path), opts)
	if err != nil {
		return nil, convertMinioErr(err)
	}
	return &minioObject{object}, nil
}

// Save save a file to minio
func (m *MinioStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	disableSignature, disableMultipart := false, false
	if m.config != nil {
		disableSignature, disableMultipart = m.config.DisableSignature, m.config.DisableMultipart
	}

	if disableMultipart && size < 0 {
		// Attempts to read everything from the source into memory. This can take a big toll on memory, and it can become a potential DoS source
		// but since we have disabled multipart upload this mean we can't really stream write anymore...
		// well, unless we have a better way to estimate the stream size, this would be a workaround

		buf := &bytes.Buffer{}
		n, err := io.Copy(buf, r)
		if err != nil {
			// I guess this would likely be EOF or OOM...?
			return -1, err
		}

		// Since we read all the data from the source, it might not be usable again,
		// so we should swap the reader location to our memory buffer
		r, size = buf, n
	}

	uploadInfo, err := m.client.PutObject(
		m.ctx,
		m.bucket,
		m.buildMinioPath(path),
		r,
		size,
		minio.PutObjectOptions{ContentType: "application/octet-stream", DisableContentSha256: disableSignature, DisableMultipart: disableMultipart},
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

func (m minioFileInfo) Sys() interface{} {
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
func (m *MinioStorage) IterateObjects(prefix string, fn func(path string, obj Object) error) error {
	opts := minio.GetObjectOptions{}
	lobjectCtx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	basePath := m.basePath
	if prefix != "" {
		basePath = m.buildMinioPath(prefix)
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
			return fn(strings.TrimPrefix(mObjInfo.Key, basePath), &minioObject{object})
		}(object, fn); err != nil {
			return convertMinioErr(err)
		}
	}
	return nil
}

func init() {
	RegisterStorageType(MinioStorageType, NewMinioStorage)
}
