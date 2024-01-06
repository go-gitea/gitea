// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var _ ObjectStorage = &GCPStorage{}

type gcpObject struct {
	Client  *storage.Client
	Object  *storage.ObjectHandle
	Context *context.Context
	Name    string
	Size    int64
	ModTime *time.Time

	Offset int64
}

func (g *gcpObject) downloadStream(p []byte) (int, error) {
	if g.Offset > g.Size {
		return 0, io.EOF
	}
	count := g.Size - g.Offset
	pl := int64(len(p))
	if pl > count {
		p = p[0:count]
	} else {
		count = pl
	}
	// Create a new Reader for the Object, that reads from Offset and for Count bytes.
	reader, err := g.Object.NewRangeReader(*g.Context, g.Offset, count)
	if err != nil {
		return 0, err // or convert to your error format
	}
	defer reader.Close()

	n, err := reader.Read(p)
	if err != nil && err != io.EOF {
		return n, err // or convert to your error format
	}

	g.Offset += int64(n)

	return n, err
}

func (g *gcpObject) Close() error {
	return nil
}

func (g *gcpObject) Read(p []byte) (int, error) {
	c, err := g.downloadStream(p)
	return c, err
}

func (g *gcpObject) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	default:
		return 0, fmt.Errorf("Seek: invalid whence")
	case io.SeekStart:
		offset += 0
	case io.SeekCurrent:
		offset += g.Offset
	case io.SeekEnd:
		offset += g.Size
	}
	if offset < 0 {
		return 0, fmt.Errorf("Seek: invalid offset")
	}
	g.Offset = offset
	return offset, nil
}

func (g *gcpObject) Stat() (os.FileInfo, error) {
	return gcpFileInfo{
		g.Name,
		g.Size,
		*g.ModTime,
	}, nil
}

// GCPStorage returns a gcp bucket storage
type GCPStorage struct {
	cfg      *setting.GoogleStorageConfig
	ctx      context.Context
	client   *storage.Client
	bucket   string
	basePath string
}

func convertGoogleErr(err error) error {
	if err == nil {
		return nil
	}

	// Convert two responses to standard analogues
	// switch errResp.Code {
	// case "NoSuchKey":
	// 	return os.ErrNotExist
	// case "AccessDenied":
	// 	return os.ErrPermission
	// }

	return err
}

// NewGoogleStorage returns a google storage
func NewGoogleStorage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	config := cfg.GoogleConfig

	log.Info("Creating Google storage at %s:%s with base path %s", config.Endpoint, config.Bucket, config.BasePath)

	opts := []option.ClientOption{}
	if config.ApplicationCredentials != "" {
		opts = append(opts, option.WithCredentialsFile(config.ApplicationCredentials))
		// Add more options here as needed
		// For example:
		// opts = append(opts, option.WithHTTPClient(yourHTTPClient))
	}
	googleClient, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, convertGoogleErr(err)
	}
	defer googleClient.Close()

	exists, err := bucketExists(ctx, googleClient, config.Bucket)
	if err != nil {
		return nil, convertGoogleErr(err)
	}
	if !exists {
		err = createBucket(ctx, googleClient, config.ProjectID, config.Bucket, config.Location)
		if err != nil {
			return nil, convertGoogleErr(err)
		}
	}

	return &GCPStorage{
		cfg:      &config,
		ctx:      ctx,
		client:   googleClient,
		bucket:   config.Bucket,
		basePath: config.BasePath,
	}, nil
}

func bucketExists(ctx context.Context, client *storage.Client, bucketName string) (bool, error) {
	bkt := client.Bucket(bucketName)
	_, err := bkt.Attrs(ctx)
	if err != nil {
		if err == storage.ErrBucketNotExist {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// createBucket creates a new bucket in the specified location (region).
func createBucket(ctx context.Context, client *storage.Client, projectID, bucketName, location string) error {
	bkt := client.Bucket(bucketName)
	bucketAttrs := &storage.BucketAttrs{
		Location: location, // Set the bucket location (region)
	}
	return bkt.Create(ctx, projectID, bucketAttrs)
}

func (g *GCPStorage) buildGooglePath(p string) string {
	p = util.PathJoinRelX(g.basePath, p)
	if p == "." {
		p = "" // gcp doesn't use dot as relative path
	}
	return p
}

func (g *GCPStorage) getObjectNameFromPath(path string) string {
	s := strings.Split(path, "/")
	return s[len(s)-1]
}

// Open opens a file
func (g *GCPStorage) Open(path string) (Object, error) {
	bkt := g.client.Bucket(g.bucket)
	obj := bkt.Object(g.buildGooglePath(path))
	attrs, err := obj.Attrs(g.ctx)
	if err != nil {
		return nil, err
	}
	return &gcpObject{
		Context: &g.ctx,
		Object:  obj,
		Name:    g.getObjectNameFromPath(path),
		Size:    attrs.Size,
		ModTime: &attrs.Updated,
	}, nil
}

// Save saves a file to gcpstorage
func (g *GCPStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	g.client.Bucket(g.bucket).Object(path).NewWriter(g.ctx)

	// Open the file that you want to upload
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	// Obtain the bucket handle
	bucket := g.client.Bucket(g.bucket)

	// Create a new object (file) in the bucket
	obj := bucket.Object(path).NewWriter(g.ctx)

	// Write the contents of the local file to GCS
	if _, err := io.Copy(obj, f); err != nil {
		return 0, fmt.Errorf("failed to write data: %v", err)
	}

	// Close the object, writing any buffered data to GCS before finalizing the object
	if err := obj.Close(); err != nil {
		return 0, fmt.Errorf("failed to close object: %v", err)
	}

	return size, nil
}

type gcpFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (g gcpFileInfo) Name() string {
	return path.Base(g.name)
}

func (g gcpFileInfo) Size() int64 {
	return g.size
}

func (g gcpFileInfo) ModTime() time.Time {
	return g.modTime
}

func (g gcpFileInfo) IsDir() bool {
	return strings.HasSuffix(g.name, "/")
}

func (g gcpFileInfo) Mode() os.FileMode {
	return os.ModePerm
}

func (g gcpFileInfo) Sys() any {
	return nil
}

// Stat returns the stat information of the object
func (g *GCPStorage) Stat(path string) (os.FileInfo, error) {
	bkt := g.client.Bucket(g.bucket)
	obj := bkt.Object(path)
	attrs, err := obj.Attrs(g.ctx)
	if err != nil {
		return nil, err
	}
	return &gcpFileInfo{
		name:    g.getObjectNameFromPath(path),
		size:    attrs.Size,
		modTime: attrs.Updated,
	}, nil
}

// Delete delete a file
func (g *GCPStorage) Delete(path string) error {
	bkt := g.client.Bucket(g.bucket)
	obj := bkt.Object(path)
	return obj.Delete(g.ctx)
}

// URL gets the redirect URL to a file. The presigned link is valid for 5 minutes.
func (g *GCPStorage) URL(path, name string) (*url.URL, error) {
	// TODO: presigned URL for gcp. May need to fetch some info found in the APP credentials file
	return nil, ErrURLNotSupported
}

// IterateObjects iterates across the objects in the gcpstorage
func (g *GCPStorage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	// Ensure the directory name ends with a '/'
	if dirName != "" && !strings.HasSuffix(dirName, "/") {
		dirName += "/"
	}

	// Get bucket handle
	bkt := g.client.Bucket(g.bucket)

	// Initialize the query to fetch the objects.
	query := &storage.Query{Prefix: dirName}
	it := bkt.Objects(g.ctx, query)

	// Loop through each object in the bucket.
	for {
		objAttrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		object := gcpObject{
			Name:    objAttrs.Name,
			Size:    objAttrs.Size,
			ModTime: &objAttrs.Updated,
		}

		// Call the provided function
		if err := fn(objAttrs.Name, &object); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	RegisterStorageType(setting.GoogleStorageType, NewGoogleStorage)
}
