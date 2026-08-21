// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

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
	ctx        context.Context
	name       string
	size       int64
	modTime    *time.Time
	offset     int64
}

func (a *azureBlobObject) Read(p []byte) (int, error) {
	// TODO: improve the performance, we can implement another interface, maybe implement io.WriteTo
	if a.offset >= a.size {
		return 0, io.EOF
	}
	count := min(int64(len(p)), a.size-a.offset)

	res, err := a.blobClient.DownloadBuffer(a.ctx, p, &blob.DownloadBufferOptions{
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
		offset = a.size + offset
	default:
		return 0, errors.New("Seek: invalid whence")
	}

	if offset > a.size {
		return 0, errors.New("Seek: invalid offset")
	} else if offset < 0 {
		return 0, errors.New("Seek: invalid offset")
	}
	a.offset = offset
	return a.offset, nil
}

func (a *azureBlobObject) Stat() (os.FileInfo, error) {
	return &azureBlobFileInfo{
		a.name,
		a.size,
		*a.modTime,
	}, nil
}

var _ ObjectStorage = &AzureBlobStorage{}

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
	return buildObjectStorePath(a.cfg.BasePath, p)
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
		ctx:        a.ctx,
		blobClient: blobClient,
		name:       a.getObjectNameFromPath(path),
		size:       *res.ContentLength,
		modTime:    res.LastModified,
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

func (a *AzureBlobStorage) getSasURL(b *blob.Client, template sas.BlobSignatureValues) (string, error) {
	urlParts, err := blob.ParseURL(b.URL())
	if err != nil {
		return "", err
	}

	var t time.Time
	if urlParts.Snapshot == "" {
		t = time.Time{}
	} else {
		t, err = time.Parse(blob.SnapshotTimeFormat, urlParts.Snapshot)
		if err != nil {
			return "", err
		}
	}

	template.ContainerName = urlParts.ContainerName
	template.BlobName = urlParts.BlobName
	template.SnapshotTime = t
	template.Version = sas.Version

	qps, err := template.SignWithSharedKey(a.credential)
	if err != nil {
		return "", err
	}

	endpoint := b.URL() + "?" + qps.Encode()

	return endpoint, nil
}

func (a *AzureBlobStorage) ServeDirectURL(storePath, name, method string, reqParams *ServeDirectOptions) (*url.URL, error) {
	blobClient := a.getBlobClient(storePath)

	startTime := time.Now().UTC()

	param := prepareServeDirectOptions(reqParams, name)

	u, err := a.getSasURL(blobClient, sas.BlobSignatureValues{
		Permissions: (&sas.BlobPermissions{
			Read:  method == http.MethodGet || method == http.MethodHead,
			Write: method == http.MethodPut,
		}).String(),
		StartTime:          startTime,
		ExpiryTime:         startTime.Add(5 * time.Minute),
		ContentDisposition: param.ContentDisposition,
		ContentType:        param.ContentType,
	})
	if err != nil {
		return nil, convertAzureBlobErr(err)
	}

	return url.Parse(u)
}

func (a *AzureBlobStorage) IterateObjects(dirName string, fn func(path string, obj Object) error) error {
	basePrefix := buildObjectStorePathPrefix(a.cfg.BasePath, "")
	dirPrefix := buildObjectStorePathPrefix(a.cfg.BasePath, dirName)
	pager := a.client.NewListBlobsFlatPager(a.cfg.Container, &container.ListBlobsFlatOptions{
		Prefix: &dirPrefix,
	})

	callback := func(object *azureBlobObject, objPath string) error {
		defer object.Close()
		return fn(objPath, object)
	}
	for pager.More() {
		resp, err := pager.NextPage(a.ctx)
		if err != nil {
			return convertAzureBlobErr(err)
		}
		for _, azureObj := range resp.Segment.BlobItems {
			objPath := strings.TrimPrefix(*azureObj.Name, basePrefix)
			objWrap := &azureBlobObject{
				ctx:        a.ctx,
				blobClient: a.getBlobClient(objPath),
				name:       *azureObj.Name,
				size:       *azureObj.Properties.ContentLength,
				modTime:    azureObj.Properties.LastModified,
			}
			if err := callback(objWrap, objPath); err != nil {
				return convertAzureBlobErr(err)
			}
		}
	}
	return nil
}

func (a *AzureBlobStorage) getBlobClient(path string) *blob.Client {
	return a.client.ServiceClient().NewContainerClient(a.cfg.Container).NewBlobClient(a.buildAzureBlobPath(path))
}

func init() {
	RegisterStorageType(setting.AzureBlobStorageType, NewAzureBlobStorage)
}
