// Copyright 2018 The Go Cloud Development Kit Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package azureblob provides a blob implementation that uses Azure Storage’s
// BlockBlob. Use OpenBucket to construct a *blob.Bucket.
//
// NOTE: SignedURLs for PUT created with this package are not fully portable;
// they will not work unless the PUT request includes a "x-ms-blob-type" header
// set to "BlockBlob".
// See https://stackoverflow.com/questions/37824136/put-on-sas-blob-url-without-specifying-x-ms-blob-type-header.
//
// URLs
//
// For blob.OpenBucket, azureblob registers for the scheme "azblob".
// The default URL opener will use credentials from the environment variables
// AZURE_STORAGE_ACCOUNT, AZURE_STORAGE_KEY, and AZURE_STORAGE_SAS_TOKEN.
// AZURE_STORAGE_ACCOUNT is required, along with one of the other two.
// To customize the URL opener, or for more details on the URL format,
// see URLOpener.
// See https://gocloud.dev/concepts/urls/ for background information.
//
// Escaping
//
// Go CDK supports all UTF-8 strings; to make this work with services lacking
// full UTF-8 support, strings must be escaped (during writes) and unescaped
// (during reads). The following escapes are performed for azureblob:
//  - Blob keys: ASCII characters 0-31, 92 ("\"), and 127 are escaped to
//    "__0x<hex>__". Additionally, the "/" in "../" and a trailing "/" in a
//    key (e.g., "foo/") are escaped in the same way.
//  - Metadata keys: Per https://docs.microsoft.com/en-us/azure/storage/blobs/storage-properties-metadata,
//    Azure only allows C# identifiers as metadata keys. Therefore, characters
//    other than "[a-z][A-z][0-9]_" are escaped using "__0x<hex>__". In addition,
//    characters "[0-9]" are escaped when they start the string.
//    URL encoding would not work since "%" is not valid.
//  - Metadata values: Escaped using URL encoding.
//
// As
//
// azureblob exposes the following types for As:
//  - Bucket: *azblob.ContainerURL
//  - Error: azblob.StorageError
//  - ListObject: azblob.BlobItem for objects, azblob.BlobPrefix for "directories"
//  - ListOptions.BeforeList: *azblob.ListBlobsSegmentOptions
//  - Reader: azblob.DownloadResponse
//  - Reader.BeforeRead: *azblob.BlockBlobURL, *azblob.BlobAccessConditions
//  - Attributes: azblob.BlobGetPropertiesResponse
//  - CopyOptions.BeforeCopy: azblob.Metadata, *azblob.ModifiedAccessConditions, *azblob.BlobAccessConditions
//  - WriterOptions.BeforeWrite: *azblob.UploadStreamToBlockBlobOptions
package azureblob

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/google/uuid"
	"github.com/google/wire"
	"gocloud.dev/blob"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"

	"gocloud.dev/internal/escape"
	"gocloud.dev/internal/useragent"
)

// Options sets options for constructing a *blob.Bucket backed by Azure Block Blob.
type Options struct {
	// Credential represents the authorizer for SignedURL.
	// Required to use SignedURL.
	Credential *azblob.SharedKeyCredential

	// SASToken can be provided along with anonymous credentials to use
	// delegated privileges.
	// See https://docs.microsoft.com/en-us/azure/storage/common/storage-dotnet-shared-access-signature-part-1#shared-access-signature-parameters.
	SASToken SASToken
}

const (
	defaultMaxDownloadRetryRequests = 3               // download retry policy (Azure default is zero)
	defaultPageSize                 = 1000            // default page size for ListPaged (Azure default is 5000)
	defaultUploadBuffers            = 5               // configure the number of rotating buffers that are used when uploading (for degree of parallelism)
	defaultUploadBlockSize          = 8 * 1024 * 1024 // configure the upload buffer size
)

func init() {
	blob.DefaultURLMux().RegisterBucket(Scheme, new(lazyCredsOpener))
}

// Set holds Wire providers for this package.
var Set = wire.NewSet(
	NewPipeline,
	wire.Struct(new(Options), "Credential", "SASToken"),
	wire.Struct(new(URLOpener), "AccountName", "Pipeline", "Options"),
)

// lazyCredsOpener obtains credentials from the environment on the first call
// to OpenBucketURL.
type lazyCredsOpener struct {
	init   sync.Once
	opener *URLOpener
	err    error
}

func (o *lazyCredsOpener) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	o.init.Do(func() {
		// Use default credential info from the environment.
		// Ignore errors, as we'll get errors from OpenBucket later.
		accountName, _ := DefaultAccountName()
		accountKey, _ := DefaultAccountKey()
		sasToken, _ := DefaultSASToken()

		o.opener, o.err = openerFromEnv(accountName, accountKey, sasToken)
	})
	if o.err != nil {
		return nil, fmt.Errorf("open bucket %v: %v", u, o.err)
	}
	return o.opener.OpenBucketURL(ctx, u)
}

// Scheme is the URL scheme gcsblob registers its URLOpener under on
// blob.DefaultMux.
const Scheme = "azblob"

// URLOpener opens Azure URLs like "azblob://mybucket".
//
// The URL host is used as the bucket name.
//
// No query parameters are supported.
type URLOpener struct {
	// AccountName must be specified.
	AccountName AccountName

	// Pipeline must be set to a non-nil value.
	Pipeline pipeline.Pipeline

	// Options specifies the options to pass to OpenBucket.
	Options Options
}

func openerFromEnv(accountName AccountName, accountKey AccountKey, sasToken SASToken) (*URLOpener, error) {
	// azblob.Credential is an interface; we will use either a SharedKeyCredential
	// or anonymous credentials. If the former, we will also fill in
	// Options.Credential so that SignedURL will work.
	var credential azblob.Credential
	var sharedKeyCred *azblob.SharedKeyCredential
	if accountKey != "" {
		var err error
		sharedKeyCred, err = NewCredential(accountName, accountKey)
		if err != nil {
			return nil, fmt.Errorf("invalid credentials %s/%s: %v", accountName, accountKey, err)
		}
		credential = sharedKeyCred
	} else {
		credential = azblob.NewAnonymousCredential()
	}
	return &URLOpener{
		AccountName: accountName,
		Pipeline:    NewPipeline(credential, azblob.PipelineOptions{}),
		Options: Options{
			Credential: sharedKeyCred,
			SASToken:   sasToken,
		},
	}, nil
}

// OpenBucketURL opens a blob.Bucket based on u.
func (o *URLOpener) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	for k := range u.Query() {
		return nil, fmt.Errorf("open bucket %v: invalid query parameter %q", u, k)
	}
	return OpenBucket(ctx, o.Pipeline, o.AccountName, u.Host, &o.Options)
}

// DefaultIdentity is a Wire provider set that provides an Azure storage
// account name, key, and SharedKeyCredential from environment variables.
var DefaultIdentity = wire.NewSet(
	DefaultAccountName,
	DefaultAccountKey,
	NewCredential,
	wire.Bind(new(azblob.Credential), new(*azblob.SharedKeyCredential)),
	wire.Value(azblob.PipelineOptions{}),
)

// SASTokenIdentity is a Wire provider set that provides an Azure storage
// account name, SASToken, and anonymous credential from environment variables.
var SASTokenIdentity = wire.NewSet(
	DefaultAccountName,
	DefaultSASToken,
	azblob.NewAnonymousCredential,
	wire.Value(azblob.PipelineOptions{}),
)

// AccountName is an Azure storage account name.
type AccountName string

// AccountKey is an Azure storage account key (primary or secondary).
type AccountKey string

// SASToken is an Azure shared access signature.
// https://docs.microsoft.com/en-us/azure/storage/common/storage-dotnet-shared-access-signature-part-1
type SASToken string

// DefaultAccountName loads the Azure storage account name from the
// AZURE_STORAGE_ACCOUNT environment variable.
func DefaultAccountName() (AccountName, error) {
	s := os.Getenv("AZURE_STORAGE_ACCOUNT")
	if s == "" {
		return "", errors.New("azureblob: environment variable AZURE_STORAGE_ACCOUNT not set")
	}
	return AccountName(s), nil
}

// DefaultAccountKey loads the Azure storage account key (primary or secondary)
// from the AZURE_STORAGE_KEY environment variable.
func DefaultAccountKey() (AccountKey, error) {
	s := os.Getenv("AZURE_STORAGE_KEY")
	if s == "" {
		return "", errors.New("azureblob: environment variable AZURE_STORAGE_KEY not set")
	}
	return AccountKey(s), nil
}

// DefaultSASToken loads a Azure SAS token from the AZURE_STORAGE_SAS_TOKEN
// environment variable.
func DefaultSASToken() (SASToken, error) {
	s := os.Getenv("AZURE_STORAGE_SAS_TOKEN")
	if s == "" {
		return "", errors.New("azureblob: environment variable AZURE_STORAGE_SAS_TOKEN not set")
	}
	return SASToken(s), nil
}

// NewCredential creates a SharedKeyCredential.
func NewCredential(accountName AccountName, accountKey AccountKey) (*azblob.SharedKeyCredential, error) {
	return azblob.NewSharedKeyCredential(string(accountName), string(accountKey))
}

// NewPipeline creates a Pipeline for making HTTP requests to Azure.
func NewPipeline(credential azblob.Credential, opts azblob.PipelineOptions) pipeline.Pipeline {
	opts.Telemetry.Value = useragent.AzureUserAgentPrefix("blob") + opts.Telemetry.Value
	return azblob.NewPipeline(credential, opts)
}

// bucket represents a Azure Storage Account Container, which handles read,
// write and delete operations on objects within it.
// See https://docs.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction.
type bucket struct {
	name         string
	pageMarkers  map[string]azblob.Marker
	serviceURL   *azblob.ServiceURL
	containerURL azblob.ContainerURL
	opts         *Options
}

// OpenBucket returns a *blob.Bucket backed by Azure Storage Account. See the package
// documentation for an example and
// https://godoc.org/github.com/Azure/azure-storage-blob-go/azblob
// for more details.
func OpenBucket(ctx context.Context, pipeline pipeline.Pipeline, accountName AccountName, containerName string, opts *Options) (*blob.Bucket, error) {
	b, err := openBucket(ctx, pipeline, accountName, containerName, opts)
	if err != nil {
		return nil, err
	}
	return blob.NewBucket(b), nil
}

func openBucket(ctx context.Context, pipeline pipeline.Pipeline, accountName AccountName, containerName string, opts *Options) (*bucket, error) {
	if pipeline == nil {
		return nil, errors.New("azureblob.OpenBucket: pipeline is required")
	}
	if accountName == "" {
		return nil, errors.New("azureblob.OpenBucket: accountName is required")
	}
	if containerName == "" {
		return nil, errors.New("azureblob.OpenBucket: containerName is required")
	}
	if opts == nil {
		opts = &Options{}
	}
	blobURL, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	if err != nil {
		return nil, err
	}
	if opts.SASToken != "" {
		// The Azure portal includes a leading "?" for the SASToken, which we
		// don't want here.
		blobURL.RawQuery = strings.TrimPrefix(string(opts.SASToken), "?")
	}
	serviceURL := azblob.NewServiceURL(*blobURL, pipeline)
	return &bucket{
		name:         containerName,
		pageMarkers:  map[string]azblob.Marker{},
		serviceURL:   &serviceURL,
		containerURL: serviceURL.NewContainerURL(containerName),
		opts:         opts,
	}, nil
}

// Close implements driver.Close.
func (b *bucket) Close() error {
	return nil
}

// Copy implements driver.Copy.
func (b *bucket) Copy(ctx context.Context, dstKey, srcKey string, opts *driver.CopyOptions) error {
	dstKey = escapeKey(dstKey, false)
	dstBlobURL := b.containerURL.NewBlobURL(dstKey)
	srcKey = escapeKey(srcKey, false)
	srcURL := b.containerURL.NewBlobURL(srcKey).URL()
	md := azblob.Metadata{}
	mac := azblob.ModifiedAccessConditions{}
	bac := azblob.BlobAccessConditions{}
	if opts.BeforeCopy != nil {
		asFunc := func(i interface{}) bool {
			switch v := i.(type) {
			case *azblob.Metadata:
				*v = md
				return true
			case **azblob.ModifiedAccessConditions:
				*v = &mac
				return true
			case **azblob.BlobAccessConditions:
				*v = &bac
				return true
			}
			return false
		}
		if err := opts.BeforeCopy(asFunc); err != nil {
			return err
		}
	}
	resp, err := dstBlobURL.StartCopyFromURL(ctx, srcURL, md, mac, bac)
	if err != nil {
		return err
	}
	copyStatus := resp.CopyStatus()
	nErrors := 0
	for copyStatus == azblob.CopyStatusPending {
		// Poll until the copy is complete.
		time.Sleep(500 * time.Millisecond)
		propertiesResp, err := dstBlobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
		if err != nil {
			// A GetProperties failure may be transient, so allow a couple
			// of them before giving up.
			nErrors++
			if ctx.Err() != nil || nErrors == 3 {
				return err
			}
		}
		copyStatus = propertiesResp.CopyStatus()
	}
	if copyStatus != azblob.CopyStatusSuccess {
		return fmt.Errorf("Copy failed with status: %s", copyStatus)
	}
	return nil
}

// Delete implements driver.Delete.
func (b *bucket) Delete(ctx context.Context, key string) error {
	key = escapeKey(key, false)
	blockBlobURL := b.containerURL.NewBlockBlobURL(key)
	_, err := blockBlobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	return err
}

// reader reads an azblob. It implements io.ReadCloser.
type reader struct {
	body  io.ReadCloser
	attrs driver.ReaderAttributes
	raw   *azblob.DownloadResponse
}

func (r *reader) Read(p []byte) (int, error) {
	return r.body.Read(p)
}
func (r *reader) Close() error {
	return r.body.Close()
}
func (r *reader) Attributes() *driver.ReaderAttributes {
	return &r.attrs
}
func (r *reader) As(i interface{}) bool {
	p, ok := i.(*azblob.DownloadResponse)
	if !ok {
		return false
	}
	*p = *r.raw
	return true
}

// NewRangeReader implements driver.NewRangeReader.
func (b *bucket) NewRangeReader(ctx context.Context, key string, offset, length int64, opts *driver.ReaderOptions) (driver.Reader, error) {
	key = escapeKey(key, false)
	blockBlobURL := b.containerURL.NewBlockBlobURL(key)
	blockBlobURLp := &blockBlobURL
	accessConditions := &azblob.BlobAccessConditions{}

	end := length
	if end < 0 {
		end = azblob.CountToEnd
	}
	if opts.BeforeRead != nil {
		asFunc := func(i interface{}) bool {
			if p, ok := i.(**azblob.BlockBlobURL); ok {
				*p = blockBlobURLp
				return true
			}
			if p, ok := i.(**azblob.BlobAccessConditions); ok {
				*p = accessConditions
				return true
			}
			return false
		}
		if err := opts.BeforeRead(asFunc); err != nil {
			return nil, err
		}
	}

	blobDownloadResponse, err := blockBlobURLp.Download(ctx, offset, end, *accessConditions, false)
	if err != nil {
		return nil, err
	}
	attrs := driver.ReaderAttributes{
		ContentType: blobDownloadResponse.ContentType(),
		Size:        getSize(blobDownloadResponse.ContentLength(), blobDownloadResponse.ContentRange()),
		ModTime:     blobDownloadResponse.LastModified(),
	}
	var body io.ReadCloser
	if length == 0 {
		body = http.NoBody
	} else {
		body = blobDownloadResponse.Body(azblob.RetryReaderOptions{MaxRetryRequests: defaultMaxDownloadRetryRequests})
	}
	return &reader{
		body:  body,
		attrs: attrs,
		raw:   blobDownloadResponse,
	}, nil
}

func getSize(contentLength int64, contentRange string) int64 {
	// Default size to ContentLength, but that's incorrect for partial-length reads,
	// where ContentLength refers to the size of the returned Body, not the entire
	// size of the blob. ContentRange has the full size.
	size := contentLength
	if contentRange != "" {
		// Sample: bytes 10-14/27 (where 27 is the full size).
		parts := strings.Split(contentRange, "/")
		if len(parts) == 2 {
			if i, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
				size = i
			}
		}
	}
	return size
}

// As implements driver.As.
func (b *bucket) As(i interface{}) bool {
	p, ok := i.(**azblob.ContainerURL)
	if !ok {
		return false
	}
	*p = &b.containerURL
	return true
}

// As implements driver.ErrorAs.
func (b *bucket) ErrorAs(err error, i interface{}) bool {
	switch v := err.(type) {
	case azblob.StorageError:
		if p, ok := i.(*azblob.StorageError); ok {
			*p = v
			return true
		}
	}
	return false
}

func (b *bucket) ErrorCode(err error) gcerrors.ErrorCode {
	serr, ok := err.(azblob.StorageError)
	switch {
	case !ok:
		return gcerrors.Unknown
	case serr.ServiceCode() == azblob.ServiceCodeBlobNotFound || serr.Response().StatusCode == 404:
		// Check and fail both the SDK ServiceCode and the Http Response Code for NotFound
		return gcerrors.NotFound
	default:
		return gcerrors.Unknown
	}
}

// Attributes implements driver.Attributes.
func (b *bucket) Attributes(ctx context.Context, key string) (*driver.Attributes, error) {
	key = escapeKey(key, false)
	blockBlobURL := b.containerURL.NewBlockBlobURL(key)
	blobPropertiesResponse, err := blockBlobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
	if err != nil {
		return nil, err
	}

	azureMD := blobPropertiesResponse.NewMetadata()
	md := make(map[string]string, len(azureMD))
	for k, v := range azureMD {
		// See the package comments for more details on escaping of metadata
		// keys & values.
		md[escape.HexUnescape(k)] = escape.URLUnescape(v)
	}
	return &driver.Attributes{
		CacheControl:       blobPropertiesResponse.CacheControl(),
		ContentDisposition: blobPropertiesResponse.ContentDisposition(),
		ContentEncoding:    blobPropertiesResponse.ContentEncoding(),
		ContentLanguage:    blobPropertiesResponse.ContentLanguage(),
		ContentType:        blobPropertiesResponse.ContentType(),
		Size:               blobPropertiesResponse.ContentLength(),
		MD5:                blobPropertiesResponse.ContentMD5(),
		ModTime:            blobPropertiesResponse.LastModified(),
		Metadata:           md,
		AsFunc: func(i interface{}) bool {
			p, ok := i.(*azblob.BlobGetPropertiesResponse)
			if !ok {
				return false
			}
			*p = *blobPropertiesResponse
			return true
		},
	}, nil
}

// ListPaged implements driver.ListPaged.
func (b *bucket) ListPaged(ctx context.Context, opts *driver.ListOptions) (*driver.ListPage, error) {
	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}

	marker := azblob.Marker{}
	if len(opts.PageToken) > 0 {
		if m, ok := b.pageMarkers[string(opts.PageToken)]; ok {
			marker = m
		}
	}

	azOpts := azblob.ListBlobsSegmentOptions{
		MaxResults: int32(pageSize),
		Prefix:     escapeKey(opts.Prefix, true),
	}
	if opts.BeforeList != nil {
		asFunc := func(i interface{}) bool {
			p, ok := i.(**azblob.ListBlobsSegmentOptions)
			if !ok {
				return false
			}
			*p = &azOpts
			return true
		}
		if err := opts.BeforeList(asFunc); err != nil {
			return nil, err
		}
	}
	listBlob, err := b.containerURL.ListBlobsHierarchySegment(ctx, marker, escapeKey(opts.Delimiter, true), azOpts)
	if err != nil {
		return nil, err
	}

	page := &driver.ListPage{}
	page.Objects = []*driver.ListObject{}
	for _, blobPrefix := range listBlob.Segment.BlobPrefixes {
		page.Objects = append(page.Objects, &driver.ListObject{
			Key:   unescapeKey(blobPrefix.Name),
			Size:  0,
			IsDir: true,
			AsFunc: func(i interface{}) bool {
				p, ok := i.(*azblob.BlobPrefix)
				if !ok {
					return false
				}
				*p = blobPrefix
				return true
			}})
	}

	for _, blobInfo := range listBlob.Segment.BlobItems {
		page.Objects = append(page.Objects, &driver.ListObject{
			Key:     unescapeKey(blobInfo.Name),
			ModTime: blobInfo.Properties.LastModified,
			Size:    *blobInfo.Properties.ContentLength,
			MD5:     blobInfo.Properties.ContentMD5,
			IsDir:   false,
			AsFunc: func(i interface{}) bool {
				p, ok := i.(*azblob.BlobItem)
				if !ok {
					return false
				}
				*p = blobInfo
				return true
			},
		})
	}

	if listBlob.NextMarker.NotDone() {
		token := uuid.New().String()
		b.pageMarkers[token] = listBlob.NextMarker
		page.NextPageToken = []byte(token)
	}
	if len(listBlob.Segment.BlobPrefixes) > 0 && len(listBlob.Segment.BlobItems) > 0 {
		sort.Slice(page.Objects, func(i, j int) bool {
			return page.Objects[i].Key < page.Objects[j].Key
		})
	}
	return page, nil
}

// SignedURL implements driver.SignedURL.
func (b *bucket) SignedURL(ctx context.Context, key string, opts *driver.SignedURLOptions) (string, error) {
	if b.opts.Credential == nil {
		return "", errors.New("to use SignedURL, you must call OpenBucket with a non-nil Options.Credential")
	}
	key = escapeKey(key, false)
	blockBlobURL := b.containerURL.NewBlockBlobURL(key)
	srcBlobParts := azblob.NewBlobURLParts(blockBlobURL.URL())

	perms := azblob.BlobSASPermissions{}
	switch opts.Method {
	case http.MethodGet:
		perms.Read = true
	case http.MethodPut:
		perms.Create = true
		perms.Write = true
	case http.MethodDelete:
		perms.Delete = true
	default:
		return "", fmt.Errorf("unsupported Method %s", opts.Method)
	}
	var err error
	srcBlobParts.SAS, err = azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    time.Now().UTC().Add(opts.Expiry),
		ContainerName: b.name,
		BlobName:      srcBlobParts.BlobName,
		Permissions:   perms.String(),
	}.NewSASQueryParameters(b.opts.Credential)
	if err != nil {
		return "", err
	}
	srcBlobURLWithSAS := srcBlobParts.URL()
	return srcBlobURLWithSAS.String(), nil
}

type writer struct {
	ctx          context.Context
	blockBlobURL *azblob.BlockBlobURL
	uploadOpts   *azblob.UploadStreamToBlockBlobOptions

	w     *io.PipeWriter
	donec chan struct{}
	err   error
}

// escapeKey does all required escaping for UTF-8 strings to work with Azure.
// isPrefix indicates whether the  key is a full key, or a prefix/delimiter.
func escapeKey(key string, isPrefix bool) string {
	return escape.HexEscape(key, func(r []rune, i int) bool {
		c := r[i]
		switch {
		// Azure does not work well with backslashes in blob names.
		case c == '\\':
			return true
		// Azure doesn't handle these characters (determined via experimentation).
		case c < 32 || c == 127:
			return true
			// Escape trailing "/" for full keys, otherwise Azure can't address them
			// consistently.
		case !isPrefix && i == len(key)-1 && c == '/':
			return true
		// For "../", escape the trailing slash.
		case i > 1 && r[i] == '/' && r[i-1] == '.' && r[i-2] == '.':
			return true
		}
		return false
	})
}

// unescapeKey reverses escapeKey.
func unescapeKey(key string) string {
	return escape.HexUnescape(key)
}

// NewTypedWriter implements driver.NewTypedWriter.
func (b *bucket) NewTypedWriter(ctx context.Context, key string, contentType string, opts *driver.WriterOptions) (driver.Writer, error) {
	key = escapeKey(key, false)
	blockBlobURL := b.containerURL.NewBlockBlobURL(key)
	if opts.BufferSize == 0 {
		opts.BufferSize = defaultUploadBlockSize
	}

	md := make(map[string]string, len(opts.Metadata))
	for k, v := range opts.Metadata {
		// See the package comments for more details on escaping of metadata
		// keys & values.
		e := escape.HexEscape(k, func(runes []rune, i int) bool {
			c := runes[i]
			switch {
			case i == 0 && c >= '0' && c <= '9':
				return true
			case escape.IsASCIIAlphanumeric(c):
				return false
			case c == '_':
				return false
			}
			return true
		})
		if _, ok := md[e]; ok {
			return nil, fmt.Errorf("duplicate keys after escaping: %q => %q", k, e)
		}
		md[e] = escape.URLEscape(v)
	}
	uploadOpts := &azblob.UploadStreamToBlockBlobOptions{
		BufferSize: opts.BufferSize,
		MaxBuffers: defaultUploadBuffers,
		Metadata:   md,
		BlobHTTPHeaders: azblob.BlobHTTPHeaders{
			CacheControl:       opts.CacheControl,
			ContentDisposition: opts.ContentDisposition,
			ContentEncoding:    opts.ContentEncoding,
			ContentLanguage:    opts.ContentLanguage,
			ContentMD5:         opts.ContentMD5,
			ContentType:        contentType,
		},
	}
	if opts.BeforeWrite != nil {
		asFunc := func(i interface{}) bool {
			p, ok := i.(**azblob.UploadStreamToBlockBlobOptions)
			if !ok {
				return false
			}
			*p = uploadOpts
			return true
		}
		if err := opts.BeforeWrite(asFunc); err != nil {
			return nil, err
		}
	}
	return &writer{
		ctx:          ctx,
		blockBlobURL: &blockBlobURL,
		uploadOpts:   uploadOpts,
		donec:        make(chan struct{}),
	}, nil
}

// Write appends p to w. User must call Close to close the w after done writing.
func (w *writer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.w == nil {
		pr, pw := io.Pipe()
		w.w = pw
		if err := w.open(pr); err != nil {
			return 0, err
		}
	}
	return w.w.Write(p)
}

func (w *writer) open(pr *io.PipeReader) error {
	go func() {
		defer close(w.donec)

		var body io.Reader
		if pr == nil {
			body = http.NoBody
		} else {
			body = pr
		}
		_, w.err = azblob.UploadStreamToBlockBlob(w.ctx, body, *w.blockBlobURL, *w.uploadOpts)
		if w.err != nil {
			if pr != nil {
				pr.CloseWithError(w.err)
			}
			return
		}
	}()
	return nil
}

// Close completes the writer and closes it. Any error occurring during write will
// be returned. If a writer is closed before any Write is called, Close will
// create an empty file at the given key.
func (w *writer) Close() error {
	if w.w == nil {
		w.open(nil)
	} else if err := w.w.Close(); err != nil {
		return err
	}
	<-w.donec
	return w.err
}
