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

// Package fileblob provides a blob implementation that uses the filesystem.
// Use OpenBucket to construct a *blob.Bucket.
//
// URLs
//
// For blob.OpenBucket, fileblob registers for the scheme "file".
// To customize the URL opener, or for more details on the URL format,
// see URLOpener.
// See https://gocloud.dev/concepts/urls/ for background information.
//
// Escaping
//
// Go CDK supports all UTF-8 strings; to make this work with services lacking
// full UTF-8 support, strings must be escaped (during writes) and unescaped
// (during reads). The following escapes are performed for fileblob:
//  - Blob keys: ASCII characters 0-31 are escaped to "__0x<hex>__".
//    If os.PathSeparator != "/", it is also escaped.
//    Additionally, the "/" in "../", the trailing "/" in "//", and a trailing
//    "/" is key names are escaped in the same way.
//    On Windows, the characters "<>:"|?*" are also escaped.
//
// As
//
// fileblob exposes the following types for As:
//  - Error: *os.PathError
package fileblob // import "gocloud.dev/blob/fileblob"

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gocloud.dev/blob"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"
	"gocloud.dev/internal/escape"
)

const defaultPageSize = 1000

func init() {
	blob.DefaultURLMux().RegisterBucket(Scheme, &URLOpener{})
}

// Scheme is the URL scheme fileblob registers its URLOpener under on
// blob.DefaultMux.
const Scheme = "file"

// URLOpener opens file bucket URLs like "file:///foo/bar/baz".
//
// The URL's host is ignored.
//
// If os.PathSeparator != "/", any leading "/" from the path is dropped
// and remaining '/' characters are converted to os.PathSeparator.
//
// The following query parameters are supported:
//
//   - base_url: the base URL to use to construct signed URLs; see URLSignerHMAC
//   - secret_key_path: path to read for the secret key used to construct signed URLs;
//     see URLSignerHMAC
//
// If either of these is provided, both must be.
//
//  - file:///a/directory
//    -> Passes "/a/directory" to OpenBucket.
//  - file://localhost/a/directory
//    -> Also passes "/a/directory".
//  - file:///c:/foo/bar on Windows.
//    -> Passes "c:\foo\bar".
//  - file://localhost/c:/foo/bar on Windows.
//    -> Also passes "c:\foo\bar".
//  - file:///a/directory?base_url=/show&secret_key_path=secret.key
//    -> Passes "/a/directory" to OpenBucket, and sets Options.URLSigner
//       to a URLSignerHMAC initialized with base URL "/show" and secret key
//       bytes read from the file "secret.key".
type URLOpener struct {
	// Options specifies the default options to pass to OpenBucket.
	Options Options
}

// OpenBucketURL opens a blob.Bucket based on u.
func (o *URLOpener) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	path := u.Path
	if os.PathSeparator != '/' {
		path = strings.TrimPrefix(path, "/")
	}
	opts, err := o.forParams(ctx, u.Query())
	if err != nil {
		return nil, fmt.Errorf("open bucket %v: %v", u, err)
	}
	return OpenBucket(filepath.FromSlash(path), opts)
}

func (o *URLOpener) forParams(ctx context.Context, q url.Values) (*Options, error) {
	for k := range q {
		if k != "base_url" && k != "secret_key_path" {
			return nil, fmt.Errorf("invalid query parameter %q", k)
		}
	}
	opts := new(Options)
	*opts = o.Options

	baseURL := q.Get("base_url")
	keyPath := q.Get("secret_key_path")
	if (baseURL == "") != (keyPath == "") {
		return nil, errors.New("must supply both base_url and secret_key_path query parameters")
	}
	if baseURL != "" {
		burl, err := url.Parse(baseURL)
		if err != nil {
			return nil, err
		}
		sk, err := ioutil.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		opts.URLSigner = NewURLSignerHMAC(burl, sk)
	}
	return opts, nil
}

// Options sets options for constructing a *blob.Bucket backed by fileblob.
type Options struct {
	// URLSigner implements signing URLs (to allow access to a resource without
	// further authorization) and verifying that a given URL is unexpired and
	// contains a signature produced by the URLSigner.
	// URLSigner is only required for utilizing the SignedURL API.
	URLSigner URLSigner
}

type bucket struct {
	dir  string
	opts *Options
}

// openBucket creates a driver.Bucket that reads and writes to dir.
// dir must exist.
func openBucket(dir string, opts *Options) (driver.Bucket, error) {
	dir = filepath.Clean(dir)
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	if opts == nil {
		opts = &Options{}
	}
	return &bucket{dir: dir, opts: opts}, nil
}

// OpenBucket creates a *blob.Bucket backed by the filesystem and rooted at
// dir, which must exist. See the package documentation for an example.
func OpenBucket(dir string, opts *Options) (*blob.Bucket, error) {
	drv, err := openBucket(dir, opts)
	if err != nil {
		return nil, err
	}
	return blob.NewBucket(drv), nil
}

func (b *bucket) Close() error {
	return nil
}

// escapeKey does all required escaping for UTF-8 strings to work the filesystem.
func escapeKey(s string) string {
	s = escape.HexEscape(s, func(r []rune, i int) bool {
		c := r[i]
		switch {
		case c < 32:
			return true
		// We're going to replace '/' with os.PathSeparator below. In order for this
		// to be reversible, we need to escape raw os.PathSeparators.
		case os.PathSeparator != '/' && c == os.PathSeparator:
			return true
		// For "../", escape the trailing slash.
		case i > 1 && c == '/' && r[i-1] == '.' && r[i-2] == '.':
			return true
		// For "//", escape the trailing slash.
		case i > 0 && c == '/' && r[i-1] == '/':
			return true
		// Escape the trailing slash in a key.
		case c == '/' && i == len(r)-1:
			return true
		// https://docs.microsoft.com/en-us/windows/desktop/fileio/naming-a-file
		case os.PathSeparator == '\\' && (c == '>' || c == '<' || c == ':' || c == '"' || c == '|' || c == '?' || c == '*'):
			return true
		}
		return false
	})
	// Replace "/" with os.PathSeparator if needed, so that the local filesystem
	// can use subdirectories.
	if os.PathSeparator != '/' {
		s = strings.Replace(s, "/", string(os.PathSeparator), -1)
	}
	return s
}

// unescapeKey reverses escapeKey.
func unescapeKey(s string) string {
	if os.PathSeparator != '/' {
		s = strings.Replace(s, string(os.PathSeparator), "/", -1)
	}
	s = escape.HexUnescape(s)
	return s
}

func (b *bucket) ErrorCode(err error) gcerrors.ErrorCode {
	switch {
	case os.IsNotExist(err):
		return gcerrors.NotFound
	default:
		return gcerrors.Unknown
	}
}

// path returns the full path for a key
func (b *bucket) path(key string) (string, error) {
	path := filepath.Join(b.dir, escapeKey(key))
	if strings.HasSuffix(path, attrsExt) {
		return "", errAttrsExt
	}
	return path, nil
}

// forKey returns the full path, os.FileInfo, and attributes for key.
func (b *bucket) forKey(key string) (string, os.FileInfo, *xattrs, error) {
	path, err := b.path(key)
	if err != nil {
		return "", nil, nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, nil, err
	}
	xa, err := getAttrs(path)
	if err != nil {
		return "", nil, nil, err
	}
	return path, info, &xa, nil
}

// ListPaged implements driver.ListPaged.
func (b *bucket) ListPaged(ctx context.Context, opts *driver.ListOptions) (*driver.ListPage, error) {

	var pageToken string
	if len(opts.PageToken) > 0 {
		pageToken = string(opts.PageToken)
	}
	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}
	// If opts.Delimiter != "", lastPrefix contains the last "directory" key we
	// added. It is used to avoid adding it again; all files in this "directory"
	// are collapsed to the single directory entry.
	var lastPrefix string

	// If the Prefix contains a "/", we can set the root of the Walk
	// to the path specified by the Prefix as any files below the path will not
	// match the Prefix.
	// Note that we use "/" explicitly and not os.PathSeparator, as the opts.Prefix
	// is in the unescaped form.
	root := b.dir
	if i := strings.LastIndex(opts.Prefix, "/"); i > -1 {
		root = filepath.Join(root, opts.Prefix[:i])
	}

	// Do a full recursive scan of the root directory.
	var result driver.ListPage
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Couldn't read this file/directory for some reason; just skip it.
			return nil
		}
		// Skip the self-generated attribute files.
		if strings.HasSuffix(path, attrsExt) {
			return nil
		}
		// os.Walk returns the root directory; skip it.
		if path == b.dir {
			return nil
		}
		// Strip the <b.dir> prefix from path; +1 is to include the separator.
		path = path[len(b.dir)+1:]
		// Unescape the path to get the key.
		key := unescapeKey(path)
		// Skip all directories. If opts.Delimiter is set, we'll create
		// pseudo-directories later.
		// Note that returning nil means that we'll still recurse into it;
		// we're just not adding a result for the directory itself.
		if info.IsDir() {
			key += "/"
			// Avoid recursing into subdirectories if the directory name already
			// doesn't match the prefix; any files in it are guaranteed not to match.
			if len(key) > len(opts.Prefix) && !strings.HasPrefix(key, opts.Prefix) {
				return filepath.SkipDir
			}
			// Similarly, avoid recursing into subdirectories if we're making
			// "directories" and all of the files in this subdirectory are guaranteed
			// to collapse to a "directory" that we've already added.
			if lastPrefix != "" && strings.HasPrefix(key, lastPrefix) {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip files/directories that don't match the Prefix.
		if !strings.HasPrefix(key, opts.Prefix) {
			return nil
		}
		var md5 []byte
		if xa, err := getAttrs(path); err == nil {
			// Note: we only have the MD5 hash for blobs that we wrote.
			// For other blobs, md5 will remain nil.
			md5 = xa.MD5
		}
		obj := &driver.ListObject{
			Key:     key,
			ModTime: info.ModTime(),
			Size:    info.Size(),
			MD5:     md5,
		}
		// If using Delimiter, collapse "directories".
		if opts.Delimiter != "" {
			// Strip the prefix, which may contain Delimiter.
			keyWithoutPrefix := key[len(opts.Prefix):]
			// See if the key still contains Delimiter.
			// If no, it's a file and we just include it.
			// If yes, it's a file in a "sub-directory" and we want to collapse
			// all files in that "sub-directory" into a single "directory" result.
			if idx := strings.Index(keyWithoutPrefix, opts.Delimiter); idx != -1 {
				prefix := opts.Prefix + keyWithoutPrefix[0:idx+len(opts.Delimiter)]
				// We've already included this "directory"; don't add it.
				if prefix == lastPrefix {
					return nil
				}
				// Update the object to be a "directory".
				obj = &driver.ListObject{
					Key:   prefix,
					IsDir: true,
				}
				lastPrefix = prefix
			}
		}
		// If there's a pageToken, skip anything before it.
		if pageToken != "" && obj.Key <= pageToken {
			return nil
		}
		// If we've already got a full page of results, set NextPageToken and stop.
		if len(result.Objects) == pageSize {
			result.NextPageToken = []byte(result.Objects[pageSize-1].Key)
			return io.EOF
		}
		result.Objects = append(result.Objects, obj)
		return nil
	})
	if err != nil && err != io.EOF {
		return nil, err
	}
	return &result, nil
}

// As implements driver.As.
func (b *bucket) As(i interface{}) bool { return false }

// As implements driver.ErrorAs.
func (b *bucket) ErrorAs(err error, i interface{}) bool {
	if perr, ok := err.(*os.PathError); ok {
		if p, ok := i.(**os.PathError); ok {
			*p = perr
			return true
		}
	}
	return false
}

// Attributes implements driver.Attributes.
func (b *bucket) Attributes(ctx context.Context, key string) (*driver.Attributes, error) {
	_, info, xa, err := b.forKey(key)
	if err != nil {
		return nil, err
	}
	return &driver.Attributes{
		CacheControl:       xa.CacheControl,
		ContentDisposition: xa.ContentDisposition,
		ContentEncoding:    xa.ContentEncoding,
		ContentLanguage:    xa.ContentLanguage,
		ContentType:        xa.ContentType,
		Metadata:           xa.Metadata,
		ModTime:            info.ModTime(),
		Size:               info.Size(),
		MD5:                xa.MD5,
	}, nil
}

// NewRangeReader implements driver.NewRangeReader.
func (b *bucket) NewRangeReader(ctx context.Context, key string, offset, length int64, opts *driver.ReaderOptions) (driver.Reader, error) {
	path, info, xa, err := b.forKey(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if opts.BeforeRead != nil {
		if err := opts.BeforeRead(func(interface{}) bool { return false }); err != nil {
			return nil, err
		}
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
	}
	r := io.Reader(f)
	if length >= 0 {
		r = io.LimitReader(r, length)
	}
	return &reader{
		r: r,
		c: f,
		attrs: driver.ReaderAttributes{
			ContentType: xa.ContentType,
			ModTime:     info.ModTime(),
			Size:        info.Size(),
		},
	}, nil
}

type reader struct {
	r     io.Reader
	c     io.Closer
	attrs driver.ReaderAttributes
}

func (r *reader) Read(p []byte) (int, error) {
	if r.r == nil {
		return 0, io.EOF
	}
	return r.r.Read(p)
}

func (r *reader) Close() error {
	if r.c == nil {
		return nil
	}
	return r.c.Close()
}

func (r *reader) Attributes() *driver.ReaderAttributes {
	return &r.attrs
}

func (r *reader) As(i interface{}) bool { return false }

// NewTypedWriter implements driver.NewTypedWriter.
func (b *bucket) NewTypedWriter(ctx context.Context, key string, contentType string, opts *driver.WriterOptions) (driver.Writer, error) {
	path, err := b.path(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
		return nil, err
	}
	f, err := ioutil.TempFile(filepath.Dir(path), "fileblob")
	if err != nil {
		return nil, err
	}
	if opts.BeforeWrite != nil {
		if err := opts.BeforeWrite(func(interface{}) bool { return false }); err != nil {
			return nil, err
		}
	}
	var metadata map[string]string
	if len(opts.Metadata) > 0 {
		metadata = opts.Metadata
	}
	attrs := xattrs{
		CacheControl:       opts.CacheControl,
		ContentDisposition: opts.ContentDisposition,
		ContentEncoding:    opts.ContentEncoding,
		ContentLanguage:    opts.ContentLanguage,
		ContentType:        contentType,
		Metadata:           metadata,
	}
	w := &writer{
		ctx:        ctx,
		f:          f,
		path:       path,
		attrs:      attrs,
		contentMD5: opts.ContentMD5,
		md5hash:    md5.New(),
	}
	return w, nil
}

type writer struct {
	ctx        context.Context
	f          *os.File
	path       string
	attrs      xattrs
	contentMD5 []byte
	// We compute the MD5 hash so that we can store it with the file attributes,
	// not for verification.
	md5hash hash.Hash
}

func (w *writer) Write(p []byte) (n int, err error) {
	if _, err := w.md5hash.Write(p); err != nil {
		return 0, err
	}
	return w.f.Write(p)
}

func (w *writer) Close() error {
	err := w.f.Close()
	if err != nil {
		return err
	}
	// Always delete the temp file. On success, it will have been renamed so
	// the Remove will fail.
	defer func() {
		_ = os.Remove(w.f.Name())
	}()

	// Check if the write was cancelled.
	if err := w.ctx.Err(); err != nil {
		return err
	}

	md5sum := w.md5hash.Sum(nil)
	w.attrs.MD5 = md5sum

	// Write the attributes file.
	if err := setAttrs(w.path, w.attrs); err != nil {
		return err
	}
	// Rename the temp file to path.
	if err := os.Rename(w.f.Name(), w.path); err != nil {
		_ = os.Remove(w.path + attrsExt)
		return err
	}
	return nil
}

// Copy implements driver.Copy.
func (b *bucket) Copy(ctx context.Context, dstKey, srcKey string, opts *driver.CopyOptions) error {
	// Note: we could use NewRangeReader here, but since we need to copy all of
	// the metadata (from xa), it's more efficient to do it directly.
	srcPath, _, xa, err := b.forKey(srcKey)
	if err != nil {
		return err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// We'll write the copy using Writer, to avoid re-implementing making of a
	// temp file, cleaning up after partial failures, etc.
	wopts := driver.WriterOptions{
		CacheControl:       xa.CacheControl,
		ContentDisposition: xa.ContentDisposition,
		ContentEncoding:    xa.ContentEncoding,
		ContentLanguage:    xa.ContentLanguage,
		Metadata:           xa.Metadata,
		BeforeWrite:        opts.BeforeCopy,
	}
	// Create a cancelable context so we can cancel the write if there are
	// problems.
	writeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	w, err := b.NewTypedWriter(writeCtx, dstKey, xa.ContentType, &wopts)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	if err != nil {
		cancel() // cancel before Close cancels the write
		w.Close()
		return err
	}
	return w.Close()
}

// Delete implements driver.Delete.
func (b *bucket) Delete(ctx context.Context, key string) error {
	path, err := b.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if err != nil {
		return err
	}
	if err = os.Remove(path + attrsExt); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SignedURL implements driver.SignedURL
func (b *bucket) SignedURL(ctx context.Context, key string, opts *driver.SignedURLOptions) (string, error) {
	if b.opts.URLSigner == nil {
		return "", errors.New("sign fileblob url: bucket does not have an Options.URLSigner")
	}
	surl, err := b.opts.URLSigner.URLFromKey(ctx, key, opts)
	if err != nil {
		return "", err
	}
	return surl.String(), nil
}

// URLSigner defines an interface for creating and verifying a signed URL for
// objects in a fileblob bucket. Signed URLs are typically used for granting
// access to an otherwise-protected resource without requiring further
// authentication, and callers should take care to restrict the creation of
// signed URLs as is appropriate for their application.
type URLSigner interface {
	// URLFromKey defines how the bucket's object key will be turned
	// into a signed URL. URLFromKey must be safe to call from multiple goroutines.
	URLFromKey(ctx context.Context, key string, opts *driver.SignedURLOptions) (*url.URL, error)

	// KeyFromURL must be able to validate a URL returned from URLFromKey.
	// KeyFromURL must only return the object if if the URL is
	// both unexpired and authentic. KeyFromURL must be safe to call from
	// multiple goroutines. Implementations of KeyFromURL should not modify
	// the URL argument.
	KeyFromURL(ctx context.Context, surl *url.URL) (string, error)
}

// URLSignerHMAC signs URLs by adding the object key, expiration time, and a
// hash-based message authentication code (HMAC) into the query parameters.
// Values of URLSignerHMAC with the same secret key will accept URLs produced by
// others as valid.
type URLSignerHMAC struct {
	baseURL   *url.URL
	secretKey []byte
}

// NewURLSignerHMAC creates a URLSignerHMAC. If the secret key is empty,
// then NewURLSignerHMAC panics.
func NewURLSignerHMAC(baseURL *url.URL, secretKey []byte) *URLSignerHMAC {
	if len(secretKey) == 0 {
		panic("creating URLSignerHMAC: secretKey is required")
	}
	uc := new(url.URL)
	*uc = *baseURL
	return &URLSignerHMAC{
		baseURL:   uc,
		secretKey: secretKey,
	}
}

// URLFromKey creates a signed URL by copying the baseURL and appending the
// object key, expiry, and signature as a query params.
func (h *URLSignerHMAC) URLFromKey(ctx context.Context, key string, opts *driver.SignedURLOptions) (*url.URL, error) {
	sURL := new(url.URL)
	*sURL = *h.baseURL

	q := sURL.Query()
	q.Set("obj", key)
	q.Set("expiry", strconv.FormatInt(time.Now().Add(opts.Expiry).Unix(), 10))
	q.Set("method", opts.Method)
	q.Set("signature", h.getMAC(q))
	sURL.RawQuery = q.Encode()

	return sURL, nil
}

func (h *URLSignerHMAC) getMAC(q url.Values) string {
	signedVals := url.Values{}
	signedVals.Set("obj", q.Get("obj"))
	signedVals.Set("expiry", q.Get("expiry"))
	signedVals.Set("method", q.Get("method"))
	msg := signedVals.Encode()

	hsh := hmac.New(sha256.New, h.secretKey)
	hsh.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(hsh.Sum(nil))
}

// KeyFromURL checks expiry and signature, and returns the object key
// only if the signed URL is both authentic and unexpired.
func (h *URLSignerHMAC) KeyFromURL(ctx context.Context, sURL *url.URL) (string, error) {
	q := sURL.Query()

	exp, err := strconv.ParseInt(q.Get("expiry"), 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", errors.New("retrieving blob key from URL: key cannot be retrieved")
	}

	if !h.checkMAC(q) {
		return "", errors.New("retrieving blob key from URL: key cannot be retrieved")
	}
	return q.Get("obj"), nil
}

func (h *URLSignerHMAC) checkMAC(q url.Values) bool {
	mac := q.Get("signature")
	expected := h.getMAC(q)
	// This compares the Base-64 encoded MACs
	return hmac.Equal([]byte(mac), []byte(expected))
}
