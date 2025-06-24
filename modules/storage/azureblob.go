// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

var _ Object = &azureBlobObject{}

type azureBlobObject struct {
	blobClient *blob.Client
	Context    context.Context
	Name       string
	Size       int64
	ModTime    *time.Time
	offset     int64
}

func (a *azureBlobObject) Read(p []byte) (int, error) {
	// TODO: improve the performance, we can implement another interface, maybe implement io.WriteTo
	if a.offset >= a.Size {
		return 0, io.EOF
	}
	count := min(int64(len(p)), a.Size-a.offset)

	res, err := a.blobClient.DownloadBuffer(a.Context, p, &blob.DownloadBufferOptions{
		Range: blob.HTTPRange{
			Offset: a.offset,
			Count:  count,
		},
	})
	if err != nil {
		return 0, convertAzureBlobErr(err)
	}
	a.offset += res

	return int(res), nil
}

func (a *azureBlobObject) Close() error {
	a.offset = 0
	return nil
}

func (a *azureBlobObject) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += a.offset
	case io.SeekEnd:
		offset = a.Size + offset
	default:
		return 0, errors.New("Seek: invalid whence")
	}

	if offset > a.Size {
		return 0, errors.New("Seek: invalid offset")
	} else if offset < 0 {
		return 0, errors.New("Seek: invalid offset")
	}
	a.offset = offset
	return a.offset, nil
}

func (a *azureBlobObject) Stat() (os.FileInfo, error) {
	return &azureBlobFileInfo{
		a.Name,
		a.Size,
		*a.ModTime,
	}, nil
}

var _ ObjectStorage = &AzureBlobStorage{}

// AzureStorage returns a azure blob storage
type AzureBlobStorage struct {
	cfg        *setting.AzureBlobStorageConfig
	ctx        context.Context
	credential *azblob.SharedKeyCredential
	client     *azblob.Client
}

func convertAzureBlobErr(err error) error {
	if err == nil {
		return nil
	}

	if bloberror.HasCode(err, bloberror.BlobNotFound) {
		return os.ErrNotExist
	}
	var respErr *azcore.ResponseError
	if !errors.As(err, &respErr) {
		return err
	}
	return fmt.Errorf("%s", respErr.ErrorCode)
}

// NewAzureBlobStorage returns a azure blob storage
func NewAzureBlobStorage(ctx context.Context, cfg *setting.Storage) (ObjectStorage, error) {
	config := cfg.AzureBlobConfig

	log.Info("Creating Azure Blob storage at %s:%s with base path %s", config.Endpoint, config.Container, config.BasePath)

	cred, err := azblob.NewSharedKeyCredential(config.AccountName, config.AccountKey)
	if err != nil {
		return nil, convertAzureBlobErr(err)
	}
	client, err := azblob.NewClientWithSharedKeyCredential(config.Endpoint, cred, &azblob.ClientOptions{})
	if err != nil {
		return nil, convertAzureBlobErr(err)
	}

	_, err = client.CreateContainer(ctx, config.Container, &container.CreateOptions{})
	if err != nil {
		// Check to see if we already own this container (which happens if you run this twice)
		if !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
			return nil, convertMinioErr(err)
		}
	}

	return &AzureBlobStorage{
		cfg:        &config,
		ctx:        ctx,
		credential: cred,
		client:     client,
	}, nil
}

func (a *AzureBlobStorage) buildAzureBlobPath(p string) string {
	p = util.PathJoinRelX(a.cfg.BasePath, p)
	if p == "." || p == "/" {
		p = "" // azure uses prefix, so path should be empty as relative path
	}
	return p
}

func (a *AzureBlobStorage) getObjectNameFromPath(path string) string {
	s := strings.Split(path, "/")
	return s[len(s)-1]
}

// Open opens a file
func (a *AzureBlobStorage) Open(path string) (Object, error) {
	blobClient := a.getBlobClient(path)
	res, err := blobClient.GetProperties(a.ctx, &blob.GetPropertiesOptions{})
	if err != nil {
		return nil, convertAzureBlobErr(err)
	}
	return &azureBlobObject{
		Context:    a.ctx,
		blobClient: blobClient,
		Name:       a.getObjectNameFromPath(path),
		Size:       *res.ContentLength,
		ModTime:    res.LastModified,
	}, nil
}

// Save saves a file to azure blob storage
func (a *AzureBlobStorage) Save(path string, r io.Reader, size int64) (int64, error) {
	rd := util.NewCountingReader(r)
	_, err := a.client.UploadStream(
		a.ctx,
		a.cfg.Container,
		a.buildAzureBlobPath(path),
		rd,
		// TODO: support set block size and concurrency
		&blockblob.UploadStreamOptions{},
	)
	if err != nil {
		return 0, convertAzureBlobErr(err)
	}
	return int64(rd.Count()), nil
}

type azureBlobFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (a azureBlobFileInfo) Name() string {
	return path.Base(a.name)
}

func (a azureBlobFileInfo) Size() int64 {
	return a.size
}

func (a azureBlobFileInfo) ModTime() time.Time {
	return a.modTime
}

func (a azureBlobFileInfo) IsDir() bool {
	return strings.HasSuffix(a.name, "/")
}

func (a azureBlobFileInfo) Mode() os.FileMode {
	return os.ModePerm
}

func (a azureBlobFileInfo) Sys() any {
	return nil
}

// Stat returns the stat information of the object
func (a *AzureBlobStorage) Stat(path string) (os.FileInfo, error) {
	blobClient := a.getBlobClient(path)
	res, err := blobClient.GetProperties(a.ctx, &blob.GetPropertiesOptions{})
	if err != nil {
		return nil, convertAzureBlobErr(err)
	}
	s := strings.Split(path, "/")
	return &azureBlobFileInfo{
		s[len(s)-1],
		*res.ContentLength,
		*res.LastModified,
	}, nil
}

// Delete delete a file
func (a *AzureBlobStorage) Delete(path string) error {
	blobClient := a.getBlobClient(path)
	_, err := blobClient.Delete(a.ctx, nil)
	return convertAzureBlobErr(err)
}

// URL gets the redirect URL to a file. The presigned link is valid for 5 minutes.
func (a *AzureBlobStorage) URL(path, name string, reqParams url.Values) (*url.URL, error) {
	blobClient := a.getBlobClient(path)

	startTime := time.Now()
	u, err := blobClient.GetSASURL(sas.BlobPermissions{
		Read: true,
	}, time.Now().Add(5*time.Minute), &blob.GetSASURLOptions{
		StartTime: &startTime,
	})
	if err != nil {
		return nil, convertAzureBlobErr(err)
	}

	return url.Parse(u)
}

// IterateObjects iterates across the objects in the azureblobstorage
func (a *AzureBlobStorage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	dirName = a.buildAzureBlobPath(dirName)
	if dirName != "" {
		dirName += "/"
	}
	pager := a.client.NewListBlobsFlatPager(a.cfg.Container, &container.ListBlobsFlatOptions{
		Prefix: &dirName,
	})
	for pager.More() {
		resp, err := pager.NextPage(a.ctx)
		if err != nil {
			return convertAzureBlobErr(err)
		}
		for _, object := range resp.Segment.BlobItems {
			blobClient := a.getBlobClient(*object.Name)
			object := &azureBlobObject{
				Context:    a.ctx,
				blobClient: blobClient,
				Name:       *object.Name,
				Size:       *object.Properties.ContentLength,
				ModTime:    object.Properties.LastModified,
			}
			if err := func(object *azureBlobObject, fn func(path string, obj Object) error) error {
				defer object.Close()
				return fn(strings.TrimPrefix(object.Name, a.cfg.BasePath), object)
			}(object, fn); err != nil {
				return convertAzureBlobErr(err)
			}
		}
	}
	return nil
}

// Delete delete a file
func (a *AzureBlobStorage) getBlobClient(path string) *blob.Client {
	return a.client.ServiceClient().NewContainerClient(a.cfg.Container).NewBlobClient(a.buildAzureBlobPath(path))
}

func init() {
	RegisterStorageType(setting.AzureBlobStorageType, NewAzureBlobStorage)
}
