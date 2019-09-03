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

// Package blob provides an easy and portable way to interact with blobs
// within a storage location. Subpackages contain driver implementations of
// blob for supported services.
//
// See https://gocloud.dev/howto/blob/ for a detailed how-to guide.
//
//
// Errors
//
// The errors returned from this package can be inspected in several ways:
//
// The Code function from gocloud.dev/gcerrors will return an error code, also
// defined in that package, when invoked on an error.
//
// The Bucket.ErrorAs method can retrieve the driver error underlying the returned
// error.
//
//
// OpenCensus Integration
//
// OpenCensus supports tracing and metric collection for multiple languages and
// backend providers. See https://opencensus.io.
//
// This API collects OpenCensus traces and metrics for the following methods:
//  - Attributes
//  - Copy
//  - Delete
//  - NewRangeReader, from creation until the call to Close. (NewReader and ReadAll
//    are included because they call NewRangeReader.)
//  - NewWriter, from creation until the call to Close.
// All trace and metric names begin with the package import path.
// The traces add the method name.
// For example, "gocloud.dev/blob/Attributes".
// The metrics are "completed_calls", a count of completed method calls by driver,
// method and status (error code); and "latency", a distribution of method latency
// by driver and method.
// For example, "gocloud.dev/blob/latency".
//
// It also collects the following metrics:
//  - gocloud.dev/blob/bytes_read: the total number of bytes read, by driver.
//  - gocloud.dev/blob/bytes_written: the total number of bytes written, by driver.
//
// To enable trace collection in your application, see "Configure Exporter" at
// https://opencensus.io/quickstart/go/tracing.
// To enable metric collection in your application, see "Exporting stats" at
// https://opencensus.io/quickstart/go/metrics.
package blob // import "gocloud.dev/blob"

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"
	"gocloud.dev/internal/gcerr"
	"gocloud.dev/internal/oc"
	"gocloud.dev/internal/openurl"
)

// Reader reads bytes from a blob.
// It implements io.ReadCloser, and must be closed after
// reads are finished.
type Reader struct {
	b        driver.Bucket
	r        driver.Reader
	key      string
	end      func(error) // called at Close to finish trace and metric collection
	provider string      // for metric collection; refers to driver package
	closed   bool
}

// Read implements io.Reader (https://golang.org/pkg/io/#Reader).
func (r *Reader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	stats.RecordWithTags(context.Background(), []tag.Mutator{tag.Upsert(oc.ProviderKey, r.provider)},
		bytesReadMeasure.M(int64(n)))
	return n, wrapError(r.b, err, r.key)
}

// Close implements io.Closer (https://golang.org/pkg/io/#Closer).
func (r *Reader) Close() error {
	r.closed = true
	err := wrapError(r.b, r.r.Close(), r.key)
	r.end(err)
	return err
}

// ContentType returns the MIME type of the blob.
func (r *Reader) ContentType() string {
	return r.r.Attributes().ContentType
}

// ModTime returns the time the blob was last modified.
func (r *Reader) ModTime() time.Time {
	return r.r.Attributes().ModTime
}

// Size returns the size of the blob content in bytes.
func (r *Reader) Size() int64 {
	return r.r.Attributes().Size
}

// As converts i to driver-specific types.
// See https://gocloud.dev/concepts/as/ for background information, the "As"
// examples in this package for examples, and the driver package
// documentation for the specific types supported for that driver.
func (r *Reader) As(i interface{}) bool {
	return r.r.As(i)
}

// Attributes contains attributes about a blob.
type Attributes struct {
	// CacheControl specifies caching attributes that services may use
	// when serving the blob.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control
	CacheControl string
	// ContentDisposition specifies whether the blob content is expected to be
	// displayed inline or as an attachment.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition
	ContentDisposition string
	// ContentEncoding specifies the encoding used for the blob's content, if any.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Encoding
	ContentEncoding string
	// ContentLanguage specifies the language used in the blob's content, if any.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Language
	ContentLanguage string
	// ContentType is the MIME type of the blob. It will not be empty.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type
	ContentType string
	// Metadata holds key/value pairs associated with the blob.
	// Keys are guaranteed to be in lowercase, even if the backend service
	// has case-sensitive keys (although note that Metadata written via
	// this package will always be lowercased). If there are duplicate
	// case-insensitive keys (e.g., "foo" and "FOO"), only one value
	// will be kept, and it is undefined which one.
	Metadata map[string]string
	// ModTime is the time the blob was last modified.
	ModTime time.Time
	// Size is the size of the blob's content in bytes.
	Size int64
	// MD5 is an MD5 hash of the blob contents or nil if not available.
	MD5 []byte

	asFunc func(interface{}) bool
}

// As converts i to driver-specific types.
// See https://gocloud.dev/concepts/as/ for background information, the "As"
// examples in this package for examples, and the driver package
// documentation for the specific types supported for that driver.
func (a *Attributes) As(i interface{}) bool {
	if a.asFunc == nil {
		return false
	}
	return a.asFunc(i)
}

// Writer writes bytes to a blob.
//
// It implements io.WriteCloser (https://golang.org/pkg/io/#Closer), and must be
// closed after all writes are done.
type Writer struct {
	b          driver.Bucket
	w          driver.Writer
	key        string
	end        func(error) // called at Close to finish trace and metric collection
	cancel     func()      // cancels the ctx provided to NewTypedWriter if contentMD5 verification fails
	contentMD5 []byte
	md5hash    hash.Hash
	provider   string // for metric collection, refers to driver package name
	closed     bool

	// These fields are non-zero values only when w is nil (not yet created).
	//
	// A ctx is stored in the Writer since we need to pass it into NewTypedWriter
	// when we finish detecting the content type of the blob and create the
	// underlying driver.Writer. This step happens inside Write or Close and
	// neither of them take a context.Context as an argument.
	//
	// All 3 fields are only initialized when we create the Writer without
	// setting the w field, and are reset to zero values after w is created.
	ctx  context.Context
	opts *driver.WriterOptions
	buf  *bytes.Buffer
}

// sniffLen is the byte size of Writer.buf used to detect content-type.
const sniffLen = 512

// Write implements the io.Writer interface (https://golang.org/pkg/io/#Writer).
//
// Writes may happen asynchronously, so the returned error can be nil
// even if the actual write eventually fails. The write is only guaranteed to
// have succeeded if Close returns no error.
func (w *Writer) Write(p []byte) (n int, err error) {
	if len(w.contentMD5) > 0 {
		if _, err := w.md5hash.Write(p); err != nil {
			return 0, err
		}
	}
	if w.w != nil {
		return w.write(p)
	}

	// If w is not yet created due to no content-type being passed in, try to sniff
	// the MIME type based on at most 512 bytes of the blob content of p.

	// Detect the content-type directly if the first chunk is at least 512 bytes.
	if w.buf.Len() == 0 && len(p) >= sniffLen {
		return w.open(p)
	}

	// Store p in w.buf and detect the content-type when the size of content in
	// w.buf is at least 512 bytes.
	w.buf.Write(p)
	if w.buf.Len() >= sniffLen {
		return w.open(w.buf.Bytes())
	}
	return len(p), nil
}

// Close closes the blob writer. The write operation is not guaranteed to have succeeded until
// Close returns with no error.
// Close may return an error if the context provided to create the Writer is
// canceled or reaches its deadline.
func (w *Writer) Close() (err error) {
	w.closed = true
	defer func() { w.end(err) }()
	if len(w.contentMD5) > 0 {
		// Verify the MD5 hash of what was written matches the ContentMD5 provided
		// by the user.
		md5sum := w.md5hash.Sum(nil)
		if !bytes.Equal(md5sum, w.contentMD5) {
			// No match! Return an error, but first cancel the context and call the
			// driver's Close function to ensure the write is aborted.
			w.cancel()
			if w.w != nil {
				_ = w.w.Close()
			}
			return gcerr.Newf(gcerr.FailedPrecondition, nil, "blob: the WriterOptions.ContentMD5 you specified (%X) did not match what was written (%X)", w.contentMD5, md5sum)
		}
	}

	defer w.cancel()
	if w.w != nil {
		return wrapError(w.b, w.w.Close(), w.key)
	}
	if _, err := w.open(w.buf.Bytes()); err != nil {
		return err
	}
	return wrapError(w.b, w.w.Close(), w.key)
}

// open tries to detect the MIME type of p and write it to the blob.
// The error it returns is wrapped.
func (w *Writer) open(p []byte) (int, error) {
	ct := http.DetectContentType(p)
	var err error
	if w.w, err = w.b.NewTypedWriter(w.ctx, w.key, ct, w.opts); err != nil {
		return 0, wrapError(w.b, err, w.key)
	}
	// Set the 3 fields needed for lazy NewTypedWriter back to zero values
	// (see the comment on Writer).
	w.buf = nil
	w.ctx = nil
	w.opts = nil
	return w.write(p)
}

func (w *Writer) write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	stats.RecordWithTags(context.Background(), []tag.Mutator{tag.Upsert(oc.ProviderKey, w.provider)},
		bytesWrittenMeasure.M(int64(n)))
	return n, wrapError(w.b, err, w.key)
}

// ListOptions sets options for listing blobs via Bucket.List.
type ListOptions struct {
	// Prefix indicates that only blobs with a key starting with this prefix
	// should be returned.
	Prefix string
	// Delimiter sets the delimiter used to define a hierarchical namespace,
	// like a filesystem with "directories". It is highly recommended that you
	// use "" or "/" as the Delimiter. Other values should work through this API,
	// but service UIs generally assume "/".
	//
	// An empty delimiter means that the bucket is treated as a single flat
	// namespace.
	//
	// A non-empty delimiter means that any result with the delimiter in its key
	// after Prefix is stripped will be returned with ListObject.IsDir = true,
	// ListObject.Key truncated after the delimiter, and zero values for other
	// ListObject fields. These results represent "directories". Multiple results
	// in a "directory" are returned as a single result.
	Delimiter string

	// BeforeList is a callback that will be called before each call to the
	// the underlying service's list functionality.
	// asFunc converts its argument to driver-specific types.
	// See https://gocloud.dev/concepts/as/ for background information.
	BeforeList func(asFunc func(interface{}) bool) error
}

// ListIterator iterates over List results.
type ListIterator struct {
	b       *Bucket
	opts    *driver.ListOptions
	page    *driver.ListPage
	nextIdx int
}

// Next returns a *ListObject for the next blob. It returns (nil, io.EOF) if
// there are no more.
func (i *ListIterator) Next(ctx context.Context) (*ListObject, error) {
	if i.page != nil {
		// We've already got a page of results.
		if i.nextIdx < len(i.page.Objects) {
			// Next object is in the page; return it.
			dobj := i.page.Objects[i.nextIdx]
			i.nextIdx++
			return &ListObject{
				Key:     dobj.Key,
				ModTime: dobj.ModTime,
				Size:    dobj.Size,
				MD5:     dobj.MD5,
				IsDir:   dobj.IsDir,
				asFunc:  dobj.AsFunc,
			}, nil
		}
		if len(i.page.NextPageToken) == 0 {
			// Done with current page, and there are no more; return io.EOF.
			return nil, io.EOF
		}
		// We need to load the next page.
		i.opts.PageToken = i.page.NextPageToken
	}
	i.b.mu.RLock()
	defer i.b.mu.RUnlock()
	if i.b.closed {
		return nil, errClosed
	}
	// Loading a new page.
	p, err := i.b.b.ListPaged(ctx, i.opts)
	if err != nil {
		return nil, wrapError(i.b.b, err, "")
	}
	i.page = p
	i.nextIdx = 0
	return i.Next(ctx)
}

// ListObject represents a single blob returned from List.
type ListObject struct {
	// Key is the key for this blob.
	Key string
	// ModTime is the time the blob was last modified.
	ModTime time.Time
	// Size is the size of the blob's content in bytes.
	Size int64
	// MD5 is an MD5 hash of the blob contents or nil if not available.
	MD5 []byte
	// IsDir indicates that this result represents a "directory" in the
	// hierarchical namespace, ending in ListOptions.Delimiter. Key can be
	// passed as ListOptions.Prefix to list items in the "directory".
	// Fields other than Key and IsDir will not be set if IsDir is true.
	IsDir bool

	asFunc func(interface{}) bool
}

// As converts i to driver-specific types.
// See https://gocloud.dev/concepts/as/ for background information, the "As"
// examples in this package for examples, and the driver package
// documentation for the specific types supported for that driver.
func (o *ListObject) As(i interface{}) bool {
	if o.asFunc == nil {
		return false
	}
	return o.asFunc(i)
}

// Bucket provides an easy and portable way to interact with blobs
// within a "bucket", including read, write, and list operations.
// To create a Bucket, use constructors found in driver subpackages.
type Bucket struct {
	b      driver.Bucket
	tracer *oc.Tracer

	// mu protects the closed variable.
	// Read locks are kept to allow holding a read lock for long-running calls,
	// and thereby prevent closing until a call finishes.
	mu     sync.RWMutex
	closed bool
}

const pkgName = "gocloud.dev/blob"

var (
	latencyMeasure      = oc.LatencyMeasure(pkgName)
	bytesReadMeasure    = stats.Int64(pkgName+"/bytes_read", "Total bytes read", stats.UnitBytes)
	bytesWrittenMeasure = stats.Int64(pkgName+"/bytes_written", "Total bytes written", stats.UnitBytes)

	// OpenCensusViews are predefined views for OpenCensus metrics.
	// The views include counts and latency distributions for API method calls,
	// and total bytes read and written.
	// See the example at https://godoc.org/go.opencensus.io/stats/view for usage.
	OpenCensusViews = append(
		oc.Views(pkgName, latencyMeasure),
		&view.View{
			Name:        pkgName + "/bytes_read",
			Measure:     bytesReadMeasure,
			Description: "Sum of bytes read from the service.",
			TagKeys:     []tag.Key{oc.ProviderKey},
			Aggregation: view.Sum(),
		},
		&view.View{
			Name:        pkgName + "/bytes_written",
			Measure:     bytesWrittenMeasure,
			Description: "Sum of bytes written to the service.",
			TagKeys:     []tag.Key{oc.ProviderKey},
			Aggregation: view.Sum(),
		})
)

// NewBucket is intended for use by drivers only. Do not use in application code.
var NewBucket = newBucket

// newBucket creates a new *Bucket based on a specific driver implementation.
// End users should use subpackages to construct a *Bucket instead of this
// function; see the package documentation for details.
func newBucket(b driver.Bucket) *Bucket {
	return &Bucket{
		b: b,
		tracer: &oc.Tracer{
			Package:        pkgName,
			Provider:       oc.ProviderName(b),
			LatencyMeasure: latencyMeasure,
		},
	}
}

// As converts i to driver-specific types.
// See https://gocloud.dev/concepts/as/ for background information, the "As"
// examples in this package for examples, and the driver package
// documentation for the specific types supported for that driver.
func (b *Bucket) As(i interface{}) bool {
	if i == nil {
		return false
	}
	return b.b.As(i)
}

// ErrorAs converts err to driver-specific types.
// ErrorAs panics if i is nil or not a pointer.
// ErrorAs returns false if err == nil.
// See https://gocloud.dev/concepts/as/ for background information.
func (b *Bucket) ErrorAs(err error, i interface{}) bool {
	return gcerr.ErrorAs(err, i, b.b.ErrorAs)
}

// ReadAll is a shortcut for creating a Reader via NewReader with nil
// ReaderOptions, and reading the entire blob.
func (b *Bucket) ReadAll(ctx context.Context, key string) (_ []byte, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, errClosed
	}
	r, err := b.NewReader(ctx, key, nil)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ioutil.ReadAll(r)
}

// List returns a ListIterator that can be used to iterate over blobs in a
// bucket, in lexicographical order of UTF-8 encoded keys. The underlying
// implementation fetches results in pages.
//
// A nil ListOptions is treated the same as the zero value.
//
// List is not guaranteed to include all recently-written blobs;
// some services are only eventually consistent.
func (b *Bucket) List(opts *ListOptions) *ListIterator {
	if opts == nil {
		opts = &ListOptions{}
	}
	dopts := &driver.ListOptions{
		Prefix:     opts.Prefix,
		Delimiter:  opts.Delimiter,
		BeforeList: opts.BeforeList,
	}
	return &ListIterator{b: b, opts: dopts}
}

// Exists returns true if a blob exists at key, false if it does not exist, or
// an error.
// It is a shortcut for calling Attributes and checking if it returns an error
// with code gcerrors.NotFound.
func (b *Bucket) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.Attributes(ctx, key)
	if err == nil {
		return true, nil
	}
	if gcerrors.Code(err) == gcerrors.NotFound {
		return false, nil
	}
	return false, err
}

// Attributes returns attributes for the blob stored at key.
//
// If the blob does not exist, Attributes returns an error for which
// gcerrors.Code will return gcerrors.NotFound.
func (b *Bucket) Attributes(ctx context.Context, key string) (_ *Attributes, err error) {
	if !utf8.ValidString(key) {
		return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: Attributes key must be a valid UTF-8 string: %q", key)
	}

	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, errClosed
	}
	ctx = b.tracer.Start(ctx, "Attributes")
	defer func() { b.tracer.End(ctx, err) }()

	a, err := b.b.Attributes(ctx, key)
	if err != nil {
		return nil, wrapError(b.b, err, key)
	}
	var md map[string]string
	if len(a.Metadata) > 0 {
		// Services are inconsistent, but at least some treat keys
		// as case-insensitive. To make the behavior consistent, we
		// force-lowercase them when writing and reading.
		md = make(map[string]string, len(a.Metadata))
		for k, v := range a.Metadata {
			md[strings.ToLower(k)] = v
		}
	}
	return &Attributes{
		CacheControl:       a.CacheControl,
		ContentDisposition: a.ContentDisposition,
		ContentEncoding:    a.ContentEncoding,
		ContentLanguage:    a.ContentLanguage,
		ContentType:        a.ContentType,
		Metadata:           md,
		ModTime:            a.ModTime,
		Size:               a.Size,
		MD5:                a.MD5,
		asFunc:             a.AsFunc,
	}, nil
}

// NewReader is a shortcut for NewRangeReader with offset=0 and length=-1.
func (b *Bucket) NewReader(ctx context.Context, key string, opts *ReaderOptions) (*Reader, error) {
	return b.newRangeReader(ctx, key, 0, -1, opts)
}

// NewRangeReader returns a Reader to read content from the blob stored at key.
// It reads at most length bytes starting at offset (>= 0).
// If length is negative, it will read till the end of the blob.
//
// If the blob does not exist, NewRangeReader returns an error for which
// gcerrors.Code will return gcerrors.NotFound. Exists is a lighter-weight way
// to check for existence.
//
// A nil ReaderOptions is treated the same as the zero value.
//
// The caller must call Close on the returned Reader when done reading.
func (b *Bucket) NewRangeReader(ctx context.Context, key string, offset, length int64, opts *ReaderOptions) (_ *Reader, err error) {
	return b.newRangeReader(ctx, key, offset, length, opts)
}

func (b *Bucket) newRangeReader(ctx context.Context, key string, offset, length int64, opts *ReaderOptions) (_ *Reader, err error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, errClosed
	}
	if offset < 0 {
		return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: NewRangeReader offset must be non-negative (%d)", offset)
	}
	if !utf8.ValidString(key) {
		return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: NewRangeReader key must be a valid UTF-8 string: %q", key)
	}
	if opts == nil {
		opts = &ReaderOptions{}
	}
	dopts := &driver.ReaderOptions{
		BeforeRead: opts.BeforeRead,
	}
	tctx := b.tracer.Start(ctx, "NewRangeReader")
	defer func() {
		// If err == nil, we handed the end closure off to the returned *Writer; it
		// will be called when the Writer is Closed.
		if err != nil {
			b.tracer.End(tctx, err)
		}
	}()
	dr, err := b.b.NewRangeReader(ctx, key, offset, length, dopts)
	if err != nil {
		return nil, wrapError(b.b, err, key)
	}
	end := func(err error) { b.tracer.End(tctx, err) }
	r := &Reader{b: b.b, r: dr, key: key, end: end, provider: b.tracer.Provider}
	_, file, lineno, ok := runtime.Caller(2)
	runtime.SetFinalizer(r, func(r *Reader) {
		if !r.closed {
			var caller string
			if ok {
				caller = fmt.Sprintf(" (%s:%d)", file, lineno)
			}
			log.Printf("A blob.Reader reading from %q was never closed%s", key, caller)
		}
	})
	return r, nil
}

// WriteAll is a shortcut for creating a Writer via NewWriter and writing p.
//
// If opts.ContentMD5 is not set, WriteAll will compute the MD5 of p and use it
// as the ContentMD5 option for the Writer it creates.
func (b *Bucket) WriteAll(ctx context.Context, key string, p []byte, opts *WriterOptions) (err error) {
	realOpts := new(WriterOptions)
	if opts != nil {
		*realOpts = *opts
	}
	if len(realOpts.ContentMD5) == 0 {
		sum := md5.Sum(p)
		realOpts.ContentMD5 = sum[:]
	}
	w, err := b.NewWriter(ctx, key, realOpts)
	if err != nil {
		return err
	}
	if _, err := w.Write(p); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// NewWriter returns a Writer that writes to the blob stored at key.
// A nil WriterOptions is treated the same as the zero value.
//
// If a blob with this key already exists, it will be replaced.
// The blob being written is not guaranteed to be readable until Close
// has been called; until then, any previous blob will still be readable.
// Even after Close is called, newly written blobs are not guaranteed to be
// returned from List; some services are only eventually consistent.
//
// The returned Writer will store ctx for later use in Write and/or Close.
// To abort a write, cancel ctx; otherwise, it must remain open until
// Close is called.
//
// The caller must call Close on the returned Writer, even if the write is
// aborted.
func (b *Bucket) NewWriter(ctx context.Context, key string, opts *WriterOptions) (_ *Writer, err error) {
	if !utf8.ValidString(key) {
		return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: NewWriter key must be a valid UTF-8 string: %q", key)
	}
	if opts == nil {
		opts = &WriterOptions{}
	}
	dopts := &driver.WriterOptions{
		CacheControl:       opts.CacheControl,
		ContentDisposition: opts.ContentDisposition,
		ContentEncoding:    opts.ContentEncoding,
		ContentLanguage:    opts.ContentLanguage,
		ContentMD5:         opts.ContentMD5,
		BufferSize:         opts.BufferSize,
		BeforeWrite:        opts.BeforeWrite,
	}
	if len(opts.Metadata) > 0 {
		// Services are inconsistent, but at least some treat keys
		// as case-insensitive. To make the behavior consistent, we
		// force-lowercase them when writing and reading.
		md := make(map[string]string, len(opts.Metadata))
		for k, v := range opts.Metadata {
			if k == "" {
				return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: WriterOptions.Metadata keys may not be empty strings")
			}
			if !utf8.ValidString(k) {
				return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: WriterOptions.Metadata keys must be valid UTF-8 strings: %q", k)
			}
			if !utf8.ValidString(v) {
				return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: WriterOptions.Metadata values must be valid UTF-8 strings: %q", v)
			}
			lowerK := strings.ToLower(k)
			if _, found := md[lowerK]; found {
				return nil, gcerr.Newf(gcerr.InvalidArgument, nil, "blob: WriterOptions.Metadata has a duplicate case-insensitive metadata key: %q", lowerK)
			}
			md[lowerK] = v
		}
		dopts.Metadata = md
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return nil, errClosed
	}
	ctx, cancel := context.WithCancel(ctx)
	tctx := b.tracer.Start(ctx, "NewWriter")
	end := func(err error) { b.tracer.End(tctx, err) }
	defer func() {
		if err != nil {
			end(err)
		}
	}()

	w := &Writer{
		b:          b.b,
		end:        end,
		cancel:     cancel,
		key:        key,
		contentMD5: opts.ContentMD5,
		md5hash:    md5.New(),
		provider:   b.tracer.Provider,
	}
	if opts.ContentType != "" {
		t, p, err := mime.ParseMediaType(opts.ContentType)
		if err != nil {
			cancel()
			return nil, err
		}
		ct := mime.FormatMediaType(t, p)
		dw, err := b.b.NewTypedWriter(ctx, key, ct, dopts)
		if err != nil {
			cancel()
			return nil, wrapError(b.b, err, key)
		}
		w.w = dw
	} else {
		// Save the fields needed to called NewTypedWriter later, once we've gotten
		// sniffLen bytes; see the comment on Writer.
		w.ctx = ctx
		w.opts = dopts
		w.buf = bytes.NewBuffer([]byte{})
	}
	_, file, lineno, ok := runtime.Caller(1)
	runtime.SetFinalizer(w, func(w *Writer) {
		if !w.closed {
			var caller string
			if ok {
				caller = fmt.Sprintf(" (%s:%d)", file, lineno)
			}
			log.Printf("A blob.Writer writing to %q was never closed%s", key, caller)
		}
	})
	return w, nil
}

// Copy the blob stored at srcKey to dstKey.
// A nil CopyOptions is treated the same as the zero value.
//
// If the source blob does not exist, Copy returns an error for which
// gcerrors.Code will return gcerrors.NotFound.
//
// If the destination blob already exists, it is overwritten.
func (b *Bucket) Copy(ctx context.Context, dstKey, srcKey string, opts *CopyOptions) (err error) {
	if !utf8.ValidString(srcKey) {
		return gcerr.Newf(gcerr.InvalidArgument, nil, "blob: Copy srcKey must be a valid UTF-8 string: %q", srcKey)
	}
	if !utf8.ValidString(dstKey) {
		return gcerr.Newf(gcerr.InvalidArgument, nil, "blob: Copy dstKey must be a valid UTF-8 string: %q", dstKey)
	}
	if opts == nil {
		opts = &CopyOptions{}
	}
	dopts := &driver.CopyOptions{
		BeforeCopy: opts.BeforeCopy,
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errClosed
	}
	ctx = b.tracer.Start(ctx, "Copy")
	defer func() { b.tracer.End(ctx, err) }()
	return wrapError(b.b, b.b.Copy(ctx, dstKey, srcKey, dopts), fmt.Sprintf("%s -> %s", srcKey, dstKey))
}

// Delete deletes the blob stored at key.
//
// If the blob does not exist, Delete returns an error for which
// gcerrors.Code will return gcerrors.NotFound.
func (b *Bucket) Delete(ctx context.Context, key string) (err error) {
	if !utf8.ValidString(key) {
		return gcerr.Newf(gcerr.InvalidArgument, nil, "blob: Delete key must be a valid UTF-8 string: %q", key)
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errClosed
	}
	ctx = b.tracer.Start(ctx, "Delete")
	defer func() { b.tracer.End(ctx, err) }()
	return wrapError(b.b, b.b.Delete(ctx, key), key)
}

// SignedURL returns a URL that can be used to GET the blob for the duration
// specified in opts.Expiry.
//
// A nil SignedURLOptions is treated the same as the zero value.
//
// It is valid to call SignedURL for a key that does not exist.
//
// If the driver does not support this functionality, SignedURL
// will return an error for which gcerrors.Code will return gcerrors.Unimplemented.
func (b *Bucket) SignedURL(ctx context.Context, key string, opts *SignedURLOptions) (string, error) {
	if !utf8.ValidString(key) {
		return "", gcerr.Newf(gcerr.InvalidArgument, nil, "blob: SignedURL key must be a valid UTF-8 string: %q", key)
	}
	if opts == nil {
		opts = &SignedURLOptions{}
	}
	if opts.Expiry < 0 {
		return "", gcerr.Newf(gcerr.InvalidArgument, nil, "blob: SignedURLOptions.Expiry must be >= 0 (%v)", opts.Expiry)
	}
	if opts.Expiry == 0 {
		opts.Expiry = DefaultSignedURLExpiry
	}
	if opts.Method == "" {
		opts.Method = http.MethodGet
	}
	switch opts.Method {
	case http.MethodGet:
	case http.MethodPut:
	case http.MethodDelete:
	default:
		return "", fmt.Errorf("unsupported SignedURLOptions.Method %q", opts.Method)
	}
	dopts := driver.SignedURLOptions{
		Expiry: opts.Expiry,
		Method: opts.Method,
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return "", errClosed
	}
	url, err := b.b.SignedURL(ctx, key, &dopts)
	return url, wrapError(b.b, err, key)
}

// Close releases any resources used for the bucket.
func (b *Bucket) Close() error {
	b.mu.Lock()
	prev := b.closed
	b.closed = true
	b.mu.Unlock()
	if prev {
		return errClosed
	}
	return wrapError(b.b, b.b.Close(), "")
}

// DefaultSignedURLExpiry is the default duration for SignedURLOptions.Expiry.
const DefaultSignedURLExpiry = 1 * time.Hour

// SignedURLOptions sets options for SignedURL.
type SignedURLOptions struct {
	// Expiry sets how long the returned URL is valid for.
	// Defaults to DefaultSignedURLExpiry.
	Expiry time.Duration
	// Method is the HTTP method that can be used on the URL; one of "GET", "PUT",
	// or "DELETE". Defaults to "GET".
	Method string
}

// ReaderOptions sets options for NewReader and NewRangeReader.
type ReaderOptions struct {
	// BeforeRead is a callback that will be called exactly once, before
	// any data is read (unless NewReader returns an error before then, in which
	// case it may not be called at all).
	//
	// asFunc converts its argument to driver-specific types.
	// See https://gocloud.dev/concepts/as/ for background information.
	BeforeRead func(asFunc func(interface{}) bool) error
}

// WriterOptions sets options for NewWriter.
type WriterOptions struct {
	// BufferSize changes the default size in bytes of the chunks that
	// Writer will upload in a single request; larger blobs will be split into
	// multiple requests.
	//
	// This option may be ignored by some drivers.
	//
	// If 0, the driver will choose a reasonable default.
	//
	// If the Writer is used to do many small writes concurrently, using a
	// smaller BufferSize may reduce memory usage.
	BufferSize int

	// CacheControl specifies caching attributes that services may use
	// when serving the blob.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Cache-Control
	CacheControl string

	// ContentDisposition specifies whether the blob content is expected to be
	// displayed inline or as an attachment.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Disposition
	ContentDisposition string

	// ContentEncoding specifies the encoding used for the blob's content, if any.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Encoding
	ContentEncoding string

	// ContentLanguage specifies the language used in the blob's content, if any.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Language
	ContentLanguage string

	// ContentType specifies the MIME type of the blob being written. If not set,
	// it will be inferred from the content using the algorithm described at
	// http://mimesniff.spec.whatwg.org/.
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Content-Type
	ContentType string

	// ContentMD5 is used as a message integrity check.
	// If len(ContentMD5) > 0, the MD5 hash of the bytes written must match
	// ContentMD5, or Close will return an error without completing the write.
	// https://tools.ietf.org/html/rfc1864
	ContentMD5 []byte

	// Metadata holds key/value strings to be associated with the blob, or nil.
	// Keys may not be empty, and are lowercased before being written.
	// Duplicate case-insensitive keys (e.g., "foo" and "FOO") will result in
	// an error.
	Metadata map[string]string

	// BeforeWrite is a callback that will be called exactly once, before
	// any data is written (unless NewWriter returns an error, in which case
	// it will not be called at all). Note that this is not necessarily during
	// or after the first Write call, as drivers may buffer bytes before
	// sending an upload request.
	//
	// asFunc converts its argument to driver-specific types.
	// See https://gocloud.dev/concepts/as/ for background information.
	BeforeWrite func(asFunc func(interface{}) bool) error
}

// CopyOptions sets options for Copy.
type CopyOptions struct {
	// BeforeCopy is a callback that will be called before the copy is
	// initiated.
	//
	// asFunc converts its argument to driver-specific types.
	// See https://gocloud.dev/concepts/as/ for background information.
	BeforeCopy func(asFunc func(interface{}) bool) error
}

// BucketURLOpener represents types that can open buckets based on a URL.
// The opener must not modify the URL argument. OpenBucketURL must be safe to
// call from multiple goroutines.
//
// This interface is generally implemented by types in driver packages.
type BucketURLOpener interface {
	OpenBucketURL(ctx context.Context, u *url.URL) (*Bucket, error)
}

// URLMux is a URL opener multiplexer. It matches the scheme of the URLs
// against a set of registered schemes and calls the opener that matches the
// URL's scheme.
// See https://gocloud.dev/concepts/urls/ for more information.
//
// The zero value is a multiplexer with no registered schemes.
type URLMux struct {
	schemes openurl.SchemeMap
}

// BucketSchemes returns a sorted slice of the registered Bucket schemes.
func (mux *URLMux) BucketSchemes() []string { return mux.schemes.Schemes() }

// ValidBucketScheme returns true iff scheme has been registered for Buckets.
func (mux *URLMux) ValidBucketScheme(scheme string) bool { return mux.schemes.ValidScheme(scheme) }

// RegisterBucket registers the opener with the given scheme. If an opener
// already exists for the scheme, RegisterBucket panics.
func (mux *URLMux) RegisterBucket(scheme string, opener BucketURLOpener) {
	mux.schemes.Register("blob", "Bucket", scheme, opener)
}

// OpenBucket calls OpenBucketURL with the URL parsed from urlstr.
// OpenBucket is safe to call from multiple goroutines.
func (mux *URLMux) OpenBucket(ctx context.Context, urlstr string) (*Bucket, error) {
	opener, u, err := mux.schemes.FromString("Bucket", urlstr)
	if err != nil {
		return nil, err
	}
	return applyPrefixParam(ctx, opener.(BucketURLOpener), u)
}

// OpenBucketURL dispatches the URL to the opener that is registered with the
// URL's scheme. OpenBucketURL is safe to call from multiple goroutines.
func (mux *URLMux) OpenBucketURL(ctx context.Context, u *url.URL) (*Bucket, error) {
	opener, err := mux.schemes.FromURL("Bucket", u)
	if err != nil {
		return nil, err
	}
	return applyPrefixParam(ctx, opener.(BucketURLOpener), u)
}

func applyPrefixParam(ctx context.Context, opener BucketURLOpener, u *url.URL) (*Bucket, error) {
	prefix := u.Query().Get("prefix")
	if prefix != "" {
		// Make a copy of u with the "prefix" parameter removed.
		urlCopy := *u
		q := urlCopy.Query()
		q.Del("prefix")
		urlCopy.RawQuery = q.Encode()
		u = &urlCopy
	}
	bucket, err := opener.OpenBucketURL(ctx, u)
	if err != nil {
		return nil, err
	}
	if prefix != "" {
		bucket = PrefixedBucket(bucket, prefix)
	}
	return bucket, nil
}

var defaultURLMux = new(URLMux)

// DefaultURLMux returns the URLMux used by OpenBucket.
//
// Driver packages can use this to register their BucketURLOpener on the mux.
func DefaultURLMux() *URLMux {
	return defaultURLMux
}

// OpenBucket opens the bucket identified by the URL given.
//
// See the URLOpener documentation in driver subpackages for
// details on supported URL formats, and https://gocloud.dev/concepts/urls/
// for more information.
//
// In addition to driver-specific query parameters, OpenBucket supports
// the following query parameters:
//
//   - prefix: wraps the resulting Bucket using PrefixedBucket with the
//             given prefix.
func OpenBucket(ctx context.Context, urlstr string) (*Bucket, error) {
	return defaultURLMux.OpenBucket(ctx, urlstr)
}

func wrapError(b driver.Bucket, err error, key string) error {
	if err == nil {
		return nil
	}
	if gcerr.DoNotWrap(err) {
		return err
	}
	msg := "blob"
	if key != "" {
		msg += fmt.Sprintf(" (key %q)", key)
	}
	return gcerr.New(b.ErrorCode(err), err, 2, msg)
}

var errClosed = gcerr.Newf(gcerr.FailedPrecondition, nil, "blob: Bucket has been closed")

// PrefixedBucket returns a *Bucket based on b with all keys modified to have
// prefix, which will usually end with a "/" to target a subdirectory in the
// bucket.
//
// bucket will be closed and no longer usable after this function returns.
func PrefixedBucket(bucket *Bucket, prefix string) *Bucket {
	bucket.mu.Lock()
	defer bucket.mu.Unlock()
	bucket.closed = true
	return NewBucket(driver.NewPrefixedBucket(bucket.b, prefix))
}
