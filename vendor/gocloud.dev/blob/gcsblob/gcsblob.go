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

// Package gcsblob provides a blob implementation that uses GCS. Use OpenBucket
// to construct a *blob.Bucket.
//
// URLs
//
// For blob.OpenBucket, gcsblob registers for the scheme "gs".
// The default URL opener will creating a connection using use default
// credentials from the environment, as described in
// https://cloud.google.com/docs/authentication/production.
// To customize the URL opener, or for more details on the URL format,
// see URLOpener.
// See https://gocloud.dev/concepts/urls/ for background information.
//
// Escaping
//
// Go CDK supports all UTF-8 strings; to make this work with services lacking
// full UTF-8 support, strings must be escaped (during writes) and unescaped
// (during reads). The following escapes are performed for gcsblob:
//  - Blob keys: ASCII characters 10 and 13 are escaped to "__0x<hex>__".
//    Additionally, the "/" in "../" is escaped in the same way.
//
// As
//
// gcsblob exposes the following types for As:
//  - Bucket: *storage.Client
//  - Error: *googleapi.Error
//  - ListObject: storage.ObjectAttrs
//  - ListOptions.BeforeList: *storage.Query
//  - Reader: *storage.Reader
//  - ReaderOptions.BeforeRead: **storage.ObjectHandle, *storage.Reader
//  - Attributes: storage.ObjectAttrs
//  - CopyOptions.BeforeCopy: *CopyObjectHandles, *storage.Copier
//  - WriterOptions.BeforeWrite: **storage.ObjectHandle, *storage.Writer
package gcsblob // import "gocloud.dev/blob/gcsblob"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/google/wire"
	"gocloud.dev/blob"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"
	"gocloud.dev/gcp"
	"gocloud.dev/internal/escape"
	"gocloud.dev/internal/useragent"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const defaultPageSize = 1000

func init() {
	blob.DefaultURLMux().RegisterBucket(Scheme, new(lazyCredsOpener))
}

// Set holds Wire providers for this package.
var Set = wire.NewSet(
	wire.Struct(new(URLOpener), "Client"),
)

// lazyCredsOpener obtains Application Default Credentials on the first call
// lazyCredsOpener obtains Application Default Credentials on the first call
// to OpenBucketURL.
type lazyCredsOpener struct {
	init   sync.Once
	opener *URLOpener
	err    error
}

func (o *lazyCredsOpener) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	o.init.Do(func() {
		creds, err := gcp.DefaultCredentials(ctx)
		if err != nil {
			o.err = err
			return
		}
		client, err := gcp.NewHTTPClient(gcp.DefaultTransport(), creds.TokenSource)
		if err != nil {
			o.err = err
			return
		}
		o.opener = &URLOpener{Client: client}
	})
	if o.err != nil {
		return nil, fmt.Errorf("open bucket %v: %v", u, o.err)
	}
	return o.opener.OpenBucketURL(ctx, u)
}

// Scheme is the URL scheme gcsblob registers its URLOpener under on
// blob.DefaultMux.
const Scheme = "gs"

// URLOpener opens GCS URLs like "gs://mybucket".
//
// The URL host is used as the bucket name.
//
// The following query parameters are supported:
//
//   - access_id: sets Options.GoogleAccessID
//   - private_key_path: path to read for Options.PrivateKey
type URLOpener struct {
	// Client must be set to a non-nil HTTP client authenticated with
	// Cloud Storage scope or equivalent.
	Client *gcp.HTTPClient

	// Options specifies the default options to pass to OpenBucket.
	Options Options
}

// OpenBucketURL opens the GCS bucket with the same name as the URL's host.
func (o *URLOpener) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	opts, err := o.forParams(ctx, u.Query())
	if err != nil {
		return nil, fmt.Errorf("open bucket %v: %v", u, err)
	}
	return OpenBucket(ctx, o.Client, u.Host, opts)
}

func (o *URLOpener) forParams(ctx context.Context, q url.Values) (*Options, error) {
	for k := range q {
		if k != "access_id" && k != "private_key_path" {
			return nil, fmt.Errorf("invalid query parameter %q", k)
		}
	}
	opts := new(Options)
	*opts = o.Options
	if accessID := q.Get("access_id"); accessID != "" {
		opts.GoogleAccessID = accessID
	}
	if keyPath := q.Get("private_key_path"); keyPath != "" {
		pk, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		opts.PrivateKey = pk
	}
	return opts, nil
}

// Options sets options for constructing a *blob.Bucket backed by GCS.
type Options struct {
	// GoogleAccessID represents the authorizer for SignedURL.
	// Required to use SignedURL.
	// See https://godoc.org/cloud.google.com/go/storage#SignedURLOptions.
	GoogleAccessID string

	// PrivateKey is the Google service account private key.
	// Exactly one of PrivateKey or SignBytes must be non-nil to use SignedURL.
	// See https://godoc.org/cloud.google.com/go/storage#SignedURLOptions.
	PrivateKey []byte

	// SignBytes is a function for implementing custom signing.
	// Exactly one of PrivateKey or SignBytes must be non-nil to use SignedURL.
	// See https://godoc.org/cloud.google.com/go/storage#SignedURLOptions.
	SignBytes func([]byte) ([]byte, error)
}

// openBucket returns a GCS Bucket that communicates using the given HTTP client.
func openBucket(ctx context.Context, client *gcp.HTTPClient, bucketName string, opts *Options) (*bucket, error) {
	if client == nil {
		return nil, errors.New("gcsblob.OpenBucket: client is required")
	}
	if bucketName == "" {
		return nil, errors.New("gcsblob.OpenBucket: bucketName is required")
	}
	// We wrap the provided http.Client to add a Go CDK User-Agent.
	c, err := storage.NewClient(ctx, option.WithHTTPClient(useragent.HTTPClient(&client.Client, "blob")))
	if err != nil {
		return nil, err
	}
	if opts == nil {
		opts = &Options{}
	}
	return &bucket{name: bucketName, client: c, opts: opts}, nil
}

// OpenBucket returns a *blob.Bucket backed by an existing GCS bucket. See the
// package documentation for an example.
func OpenBucket(ctx context.Context, client *gcp.HTTPClient, bucketName string, opts *Options) (*blob.Bucket, error) {
	drv, err := openBucket(ctx, client, bucketName, opts)
	if err != nil {
		return nil, err
	}
	return blob.NewBucket(drv), nil
}

// bucket represents a GCS bucket, which handles read, write and delete operations
// on objects within it.
type bucket struct {
	name   string
	client *storage.Client
	opts   *Options
}

var emptyBody = ioutil.NopCloser(strings.NewReader(""))

// reader reads a GCS object. It implements driver.Reader.
type reader struct {
	body  io.ReadCloser
	attrs driver.ReaderAttributes
	raw   *storage.Reader
}

func (r *reader) Read(p []byte) (int, error) {
	return r.body.Read(p)
}

// Close closes the reader itself. It must be called when done reading.
func (r *reader) Close() error {
	return r.body.Close()
}

func (r *reader) Attributes() *driver.ReaderAttributes {
	return &r.attrs
}

func (r *reader) As(i interface{}) bool {
	p, ok := i.(**storage.Reader)
	if !ok {
		return false
	}
	*p = r.raw
	return true
}

func (b *bucket) ErrorCode(err error) gcerrors.ErrorCode {
	if err == storage.ErrObjectNotExist {
		return gcerrors.NotFound
	}
	if gerr, ok := err.(*googleapi.Error); ok {
		switch gerr.Code {
		case http.StatusNotFound:
			return gcerrors.NotFound
		case http.StatusPreconditionFailed:
			return gcerrors.FailedPrecondition
		}
	}
	return gcerrors.Unknown
}

func (b *bucket) Close() error {
	return nil
}

// ListPaged implements driver.ListPaged.
func (b *bucket) ListPaged(ctx context.Context, opts *driver.ListOptions) (*driver.ListPage, error) {
	bkt := b.client.Bucket(b.name)
	query := &storage.Query{
		Prefix:    escapeKey(opts.Prefix),
		Delimiter: escapeKey(opts.Delimiter),
	}
	if opts.BeforeList != nil {
		asFunc := func(i interface{}) bool {
			p, ok := i.(**storage.Query)
			if !ok {
				return false
			}
			*p = query
			return true
		}
		if err := opts.BeforeList(asFunc); err != nil {
			return nil, err
		}
	}
	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	iter := bkt.Objects(ctx, query)
	pager := iterator.NewPager(iter, pageSize, string(opts.PageToken))
	var objects []*storage.ObjectAttrs
	nextPageToken, err := pager.NextPage(&objects)
	if err != nil {
		return nil, err
	}
	page := driver.ListPage{NextPageToken: []byte(nextPageToken)}
	if len(objects) > 0 {
		page.Objects = make([]*driver.ListObject, len(objects))
		for i, obj := range objects {
			asFunc := func(i interface{}) bool {
				p, ok := i.(*storage.ObjectAttrs)
				if !ok {
					return false
				}
				*p = *obj
				return true
			}
			if obj.Prefix == "" {
				// Regular blob.
				page.Objects[i] = &driver.ListObject{
					Key:     unescapeKey(obj.Name),
					ModTime: obj.Updated,
					Size:    obj.Size,
					MD5:     obj.MD5,
					AsFunc:  asFunc,
				}
			} else {
				// "Directory".
				page.Objects[i] = &driver.ListObject{
					Key:    unescapeKey(obj.Prefix),
					IsDir:  true,
					AsFunc: asFunc,
				}
			}
		}
		// GCS always returns "directories" at the end; sort them.
		sort.Slice(page.Objects, func(i, j int) bool {
			return page.Objects[i].Key < page.Objects[j].Key
		})
	}
	return &page, nil
}

// As implements driver.As.
func (b *bucket) As(i interface{}) bool {
	p, ok := i.(**storage.Client)
	if !ok {
		return false
	}
	*p = b.client
	return true
}

// As implements driver.ErrorAs.
func (b *bucket) ErrorAs(err error, i interface{}) bool {
	switch v := err.(type) {
	case *googleapi.Error:
		if p, ok := i.(**googleapi.Error); ok {
			*p = v
			return true
		}
	}
	return false
}

// Attributes implements driver.Attributes.
func (b *bucket) Attributes(ctx context.Context, key string) (*driver.Attributes, error) {
	key = escapeKey(key)
	bkt := b.client.Bucket(b.name)
	obj := bkt.Object(key)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, err
	}
	return &driver.Attributes{
		CacheControl:       attrs.CacheControl,
		ContentDisposition: attrs.ContentDisposition,
		ContentEncoding:    attrs.ContentEncoding,
		ContentLanguage:    attrs.ContentLanguage,
		ContentType:        attrs.ContentType,
		Metadata:           attrs.Metadata,
		ModTime:            attrs.Updated,
		Size:               attrs.Size,
		MD5:                attrs.MD5,
		AsFunc: func(i interface{}) bool {
			p, ok := i.(*storage.ObjectAttrs)
			if !ok {
				return false
			}
			*p = *attrs
			return true
		},
	}, nil
}

// NewRangeReader implements driver.NewRangeReader.
func (b *bucket) NewRangeReader(ctx context.Context, key string, offset, length int64, opts *driver.ReaderOptions) (driver.Reader, error) {
	key = escapeKey(key)
	bkt := b.client.Bucket(b.name)
	obj := bkt.Object(key)

	// Add an extra level of indirection so that BeforeRead can replace obj
	// if needed. For example, ObjectHandle.If returns a new ObjectHandle.
	// Also, make the Reader lazily in case this replacement happens.
	objp := &obj
	makeReader := func() (*storage.Reader, error) {
		return (*objp).NewRangeReader(ctx, offset, length)
	}

	var r *storage.Reader
	var rerr error
	madeReader := false
	if opts.BeforeRead != nil {
		asFunc := func(i interface{}) bool {
			if p, ok := i.(***storage.ObjectHandle); ok && !madeReader {
				*p = objp
				return true
			}
			if p, ok := i.(**storage.Reader); ok {
				if !madeReader {
					r, rerr = makeReader()
					madeReader = true
				}
				*p = r
				return true
			}
			return false
		}
		if err := opts.BeforeRead(asFunc); err != nil {
			return nil, err
		}
	}
	if !madeReader {
		r, rerr = makeReader()
	}
	if rerr != nil {
		return nil, rerr
	}
	modTime, _ := r.LastModified()
	return &reader{
		body: r,
		attrs: driver.ReaderAttributes{
			ContentType: r.ContentType(),
			ModTime:     modTime,
			Size:        r.Size(),
		},
		raw: r,
	}, nil
}

// escapeKey does all required escaping for UTF-8 strings to work with GCS.
func escapeKey(key string) string {
	return escape.HexEscape(key, func(r []rune, i int) bool {
		switch {
		// GCS doesn't handle these characters (determined via experimentation).
		case r[i] == 10 || r[i] == 13:
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
	key = escapeKey(key)
	bkt := b.client.Bucket(b.name)
	obj := bkt.Object(key)

	// Add an extra level of indirection so that BeforeWrite can replace obj
	// if needed. For example, ObjectHandle.If returns a new ObjectHandle.
	// Also, make the Writer lazily in case this replacement happens.
	objp := &obj
	makeWriter := func() *storage.Writer {
		w := (*objp).NewWriter(ctx)
		w.CacheControl = opts.CacheControl
		w.ContentDisposition = opts.ContentDisposition
		w.ContentEncoding = opts.ContentEncoding
		w.ContentLanguage = opts.ContentLanguage
		w.ContentType = contentType
		w.ChunkSize = bufferSize(opts.BufferSize)
		w.Metadata = opts.Metadata
		w.MD5 = opts.ContentMD5
		return w
	}

	var w *storage.Writer
	if opts.BeforeWrite != nil {
		asFunc := func(i interface{}) bool {
			if p, ok := i.(***storage.ObjectHandle); ok && w == nil {
				*p = objp
				return true
			}
			if p, ok := i.(**storage.Writer); ok {
				if w == nil {
					w = makeWriter()
				}
				*p = w
				return true
			}
			return false
		}
		if err := opts.BeforeWrite(asFunc); err != nil {
			return nil, err
		}
	}
	if w == nil {
		w = makeWriter()
	}
	return w, nil
}

// CopyObjectHandles holds the ObjectHandles for the destination and source
// of a Copy. It is used by the BeforeCopy As hook.
type CopyObjectHandles struct {
	Dst, Src *storage.ObjectHandle
}

// Copy implements driver.Copy.
func (b *bucket) Copy(ctx context.Context, dstKey, srcKey string, opts *driver.CopyOptions) error {
	dstKey = escapeKey(dstKey)
	srcKey = escapeKey(srcKey)
	bkt := b.client.Bucket(b.name)

	// Add an extra level of indirection so that BeforeCopy can replace the
	// dst or src ObjectHandles if needed.
	// Also, make the Copier lazily in case this replacement happens.
	handles := CopyObjectHandles{
		Dst: bkt.Object(dstKey),
		Src: bkt.Object(srcKey),
	}
	makeCopier := func() *storage.Copier {
		return handles.Dst.CopierFrom(handles.Src)
	}

	var copier *storage.Copier
	if opts.BeforeCopy != nil {
		asFunc := func(i interface{}) bool {
			if p, ok := i.(**CopyObjectHandles); ok && copier == nil {
				*p = &handles
				return true
			}
			if p, ok := i.(**storage.Copier); ok {
				if copier == nil {
					copier = makeCopier()
				}
				*p = copier
				return true
			}
			return false
		}
		if err := opts.BeforeCopy(asFunc); err != nil {
			return err
		}
	}
	if copier == nil {
		copier = makeCopier()
	}
	_, err := copier.Run(ctx)
	return err
}

// Delete implements driver.Delete.
func (b *bucket) Delete(ctx context.Context, key string) error {
	key = escapeKey(key)
	bkt := b.client.Bucket(b.name)
	obj := bkt.Object(key)
	return obj.Delete(ctx)
}

func (b *bucket) SignedURL(ctx context.Context, key string, dopts *driver.SignedURLOptions) (string, error) {
	if b.opts.GoogleAccessID == "" || (b.opts.PrivateKey == nil && b.opts.SignBytes == nil) {
		return "", errors.New("to use SignedURL, you must call OpenBucket with a valid Options.GoogleAccessID and exactly one of Options.PrivateKey or Options.SignBytes")
	}
	key = escapeKey(key)
	opts := &storage.SignedURLOptions{
		Expires:        time.Now().Add(dopts.Expiry),
		Method:         dopts.Method,
		GoogleAccessID: b.opts.GoogleAccessID,
		PrivateKey:     b.opts.PrivateKey,
		SignBytes:      b.opts.SignBytes,
	}
	return storage.SignedURL(b.name, key, opts)
}

func bufferSize(size int) int {
	if size == 0 {
		return googleapi.DefaultUploadChunkSize
	} else if size > 0 {
		return size
	}
	return 0 // disable buffering
}
