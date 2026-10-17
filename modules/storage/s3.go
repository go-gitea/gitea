// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var _ ObjectStorage = &S3Storage{}

const unknownSizePartSize = 1024 * 1024 * 16 // same as minio-go's minPartSize

type s3Object struct {
	*minio.Object
}

func (m *s3Object) Stat() (os.FileInfo, error) {
	oi, err := m.Object.Stat()
	if err != nil {
		return nil, convertS3Err(err)
	}

	return &objectFileInfo{path.Base(oi.Key), oi.Size, oi.LastModified}, nil
}

// minio reports a missing key on the first Read, ReadAt or Seek rather than on Open, so all
// of them convert it like Stat does.
func (m *s3Object) Read(p []byte) (int, error) {
	n, err := m.Object.Read(p)
	return n, convertS3Err(err)
}

func (m *s3Object) ReadAt(p []byte, off int64) (int, error) {
	n, err := m.Object.ReadAt(p, off)
	return n, convertS3Err(err)
}

func (m *s3Object) Seek(offset int64, whence int) (int64, error) {
	n, err := m.Object.Seek(offset, whence)
	return n, convertS3Err(err)
}

// S3Storage returns an S3 bucket storage
type S3Storage struct {
	cfg      *setting.S3StorageConfig
	ctx      context.Context
	client   *minio.Client
	bucket   string
	basePath string
}

func convertS3Err(err error, optMsg ...string) error {
	if err == nil {
		return nil
	}

	wrapErr := func(err error) error {
		if len(optMsg) == 0 {
			return err
		}
		return fmt.Errorf("%s: %w", optMsg[0], err)
	}

	errResp, ok := errors.AsType[minio.ErrorResponse](err)
	if !ok {
		return wrapErr(err)
	}

	// Convert two responses to standard analogues
	switch errResp.Code {
	case "NoSuchKey":
		return wrapErr(fs.ErrNotExist)
	case "AccessDenied":
		return wrapErr(fs.ErrPermission)
	}

	return wrapErr(err)
}

// NewS3Storage returns an S3 storage
func NewS3Storage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	config := cfg.S3Config
	log.Info("Creating S3 storage at %s:%s with base path %s", config.Endpoint, config.Bucket, config.BasePath)
	if config.ChecksumAlgorithm != "" && config.ChecksumAlgorithm != "default" && config.ChecksumAlgorithm != "md5" {
		return nil, fmt.Errorf("invalid S3 checksum algorithm: %s", config.ChecksumAlgorithm)
	}

	var lookup minio.BucketLookupType
	switch config.BucketLookUpType {
	case "auto", "":
		lookup = minio.BucketLookupAuto
	case "dns":
		lookup = minio.BucketLookupDNS
	case "path":
		lookup = minio.BucketLookupPath
	default:
		return nil, fmt.Errorf("invalid S3 bucket lookup type: %s", config.BucketLookUpType)
	}

	// The request error message is something like:
	// * "The request signature we calculated does not match the signature you provided. Check your key and signing method."
	// It doesn't contain useful information to site admin, so here we wrap the error with our error message
	// to tell the site admin what is the problem.
	makeErrMsg := func(hint string) string {
		return fmt.Sprintf("ObjectStorage.%s: endpoint=%s, location=%s, bucket=%s", hint, config.Endpoint, config.Location, config.Bucket)
	}
	s3Client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:        buildS3Credentials(config),
		Secure:       config.UseSSL,
		Transport:    &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: config.InsecureSkipVerify}},
		Region:       config.Location,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, convertS3Err(err, makeErrMsg("NewClient"))
	}

	// Check to see if we already own this bucket
	exists, err := s3Client.BucketExists(ctx, config.Bucket)
	if err != nil {
		return nil, convertS3Err(err, makeErrMsg("BucketExists"))
	}

	// If the bucket doesn't exist, try to create one
	if !exists {
		if err := s3Client.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{
			Region: config.Location,
		}); err != nil {
			return nil, convertS3Err(err, makeErrMsg("MakeBucket"))
		}
	}

	return &S3Storage{
		cfg:      &config,
		ctx:      ctx,
		client:   s3Client,
		bucket:   config.Bucket,
		basePath: config.BasePath,
	}, nil
}

func (m *S3Storage) buildS3Path(p string) string {
	return buildObjectStorePath(m.basePath, p)
}

func (m *S3Storage) buildS3DirPrefix(p string) string {
	return buildObjectStorePathPrefix(m.basePath, p)
}

func buildS3Credentials(config setting.S3StorageConfig) *credentials.Credentials {
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
			// passing in an empty Endpoint lets the IAM Provider
			// decide which endpoint to resolve internally
			Endpoint: config.IamEndpoint,
			Client: &http.Client{
				Transport: http.DefaultTransport,
			},
		},
	}
	return credentials.NewChainCredentials(chain)
}

// Open opens a file
func (m *S3Storage) Open(path string) (Object, error) {
	opts := minio.GetObjectOptions{}
	object, err := m.client.GetObject(m.ctx, m.bucket, m.buildS3Path(path), opts)
	if err != nil {
		return nil, convertS3Err(err)
	}
	return &s3Object{object}, nil
}

// Save saves a file to S3
func (m *S3Storage) Save(path string, r io.Reader, size int64) (int64, error) {
	uploadInfo, err := m.client.PutObject(
		m.ctx,
		m.bucket,
		m.buildS3Path(path),
		r,
		size,
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
			// some storages like:
			// * https://developers.cloudflare.com/r2/api/s3/api/
			// * https://www.backblaze.com/b2/docs/s3_compatible_api.html
			// do not support "x-amz-checksum-algorithm" header, so use legacy MD5 checksum
			SendContentMd5: m.cfg.ChecksumAlgorithm == "md5",

			// with an unknown size (-1) minio-go assumes a 5TiB object and buffers a 528MiB part for it, even
			// for a payload of a few KiB, so pin the part size there, a known size derives its own
			PartSize: util.Iif[uint64](size < 0, unknownSizePartSize, 0),
		},
	)
	if err != nil {
		return 0, convertS3Err(err)
	}
	return uploadInfo.Size, nil
}

// Stat returns the stat information of the object
func (m *S3Storage) Stat(p string) (os.FileInfo, error) {
	info, err := m.client.StatObject(
		m.ctx,
		m.bucket,
		m.buildS3Path(p),
		minio.StatObjectOptions{},
	)
	if err != nil {
		return nil, convertS3Err(err)
	}
	return &objectFileInfo{path.Base(info.Key), info.Size, info.LastModified}, nil
}

// Delete delete a file
func (m *S3Storage) Delete(path string) error {
	err := m.client.RemoveObject(m.ctx, m.bucket, m.buildS3Path(path), minio.RemoveObjectOptions{})

	return convertS3Err(err)
}

func (m *S3Storage) ServeDirectURL(storePath, name, method string, opt *ServeDirectOptions) (*url.URL, error) {
	reqParams := url.Values{}

	param := prepareServeDirectOptions(opt, name)
	// minio does not ignore empty params
	if param.ContentType != "" {
		reqParams.Set("response-content-type", param.ContentType)
	}
	if param.ContentDisposition != "" {
		reqParams.Set("response-content-disposition", param.ContentDisposition)
	}

	expires := 5 * time.Minute
	if method == http.MethodHead {
		u, err := m.client.PresignedHeadObject(m.ctx, m.bucket, m.buildS3Path(storePath), expires, reqParams)
		return u, convertS3Err(err)
	}
	u, err := m.client.PresignedGetObject(m.ctx, m.bucket, m.buildS3Path(storePath), expires, reqParams)
	return u, convertS3Err(err)
}

func (m *S3Storage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	basePrefix := m.buildS3DirPrefix("")
	dirPrefix := m.buildS3DirPrefix(dirName)
	callback := func(object *minio.Object, objPath string) error {
		defer object.Close()
		return fn(objPath, &s3Object{object})
	}

	ctxList, ctxListCancel := context.WithCancel(m.ctx)
	defer ctxListCancel() // ListObjectsIter: make sure to cancel the passed context, without that you might leak coroutines

	getOpts := minio.GetObjectOptions{}
	listOpts := minio.ListObjectsOptions{Prefix: dirPrefix, Recursive: true}
	for mObjInfo := range m.client.ListObjectsIter(ctxList, m.bucket, listOpts) {
		object, err := m.client.GetObject(m.ctx, m.bucket, mObjInfo.Key, getOpts)
		if err != nil {
			return convertS3Err(err)
		}
		objPath := strings.TrimPrefix(mObjInfo.Key, basePrefix)
		if err := callback(object, objPath); err != nil {
			return convertS3Err(err)
		}
	}
	return nil
}

func init() {
	RegisterStorageType(setting.S3StorageType, NewS3Storage)
}
