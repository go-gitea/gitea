// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gzip

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"gitea.com/macaron/macaron"
	"github.com/klauspost/compress/gzip"
)

const (
	acceptEncodingHeader  = "Accept-Encoding"
	contentEncodingHeader = "Content-Encoding"
	contentLengthHeader   = "Content-Length"
	contentTypeHeader     = "Content-Type"
	rangeHeader           = "Range"
	varyHeader            = "Vary"
)

const (
	// MinSize is the minimum size of content we will compress
	MinSize = 1400
)

// noopClosers are io.Writers with a shim to prevent early closure
type noopCloser struct {
	io.Writer
}

func (noopCloser) Close() error { return nil }

// WriterPool is a gzip writer pool to reduce workload on creation of
// gzip writers
type WriterPool struct {
	pool             sync.Pool
	compressionLevel int
}

// NewWriterPool creates a new pool
func NewWriterPool(compressionLevel int) *WriterPool {
	return &WriterPool{pool: sync.Pool{
		// New will return nil, we'll manage the creation of new
		// writers in the middleware
		New: func() interface{} { return nil },
	},
		compressionLevel: compressionLevel}
}

// Get a writer from the pool - or create one if not available
func (wp *WriterPool) Get(rw macaron.ResponseWriter) *gzip.Writer {
	ret := wp.pool.Get()
	if ret == nil {
		ret, _ = gzip.NewWriterLevel(rw, wp.compressionLevel)
	} else {
		ret.(*gzip.Writer).Reset(rw)
	}
	return ret.(*gzip.Writer)
}

// Put returns a writer to the pool
func (wp *WriterPool) Put(w *gzip.Writer) {
	wp.pool.Put(w)
}

var writerPool WriterPool

// Options represents the configuration for the gzip middleware
type Options struct {
	CompressionLevel int
}

func validateCompressionLevel(level int) bool {
	return level == gzip.DefaultCompression ||
		level == gzip.ConstantCompression ||
		(level >= gzip.BestSpeed && level <= gzip.BestCompression)
}

func validate(options []Options) Options {
	// Default to level 4 compression (Best results seem to be between 4 and 6)
	opt := Options{CompressionLevel: 4}
	if len(options) > 0 {
		opt = options[0]
	}
	if !validateCompressionLevel(opt.CompressionLevel) {
		opt.CompressionLevel = 4
	}
	return opt
}

// Middleware creates a macaron.Handler to proxy the response
func Middleware(options ...Options) macaron.Handler {
	opt := validate(options)
	writerPool = *NewWriterPool(opt.CompressionLevel)
	regex := regexp.MustCompile(`bytes=(\d+)\-.*`)

	return func(ctx *macaron.Context) {
		// If the client won't accept gzip or x-gzip don't compress
		if !strings.Contains(ctx.Req.Header.Get(acceptEncodingHeader), "gzip") &&
			!strings.Contains(ctx.Req.Header.Get(acceptEncodingHeader), "x-gzip") {
			return
		}

		// If the client is asking for a specific range of bytes - don't compress
		if rangeHdr := ctx.Req.Header.Get(rangeHeader); rangeHdr != "" {

			match := regex.FindStringSubmatch(rangeHdr)
			if len(match) > 1 {
				return
			}
		}

		// OK we should proxy the response writer
		// We are still not necessarily going to compress...
		proxyWriter := &ProxyResponseWriter{
			internal: ctx.Resp,
		}
		defer proxyWriter.Close()

		ctx.Resp = proxyWriter
		ctx.MapTo(proxyWriter, (*http.ResponseWriter)(nil))

		// Check if render middleware has been registered,
		// if yes, we need to modify ResponseWriter for it as well.
		if _, ok := ctx.Render.(*macaron.DummyRender); !ok {
			ctx.Render.SetResponseWriter(proxyWriter)
		}

		ctx.Next()
		ctx.Resp = proxyWriter.internal
	}
}

// ProxyResponseWriter is a wrapped macaron ResponseWriter that may compress its contents
type ProxyResponseWriter struct {
	writer   io.WriteCloser
	internal macaron.ResponseWriter
	stopped  bool

	code int
	buf  []byte
}

// Header returns the header map
func (proxy *ProxyResponseWriter) Header() http.Header {
	return proxy.internal.Header()
}

// Status returns the status code of the response or 0 if the response has not been written.
func (proxy *ProxyResponseWriter) Status() int {
	if proxy.code != 0 {
		return proxy.code
	}
	return proxy.internal.Status()
}

// Written returns whether or not the ResponseWriter has been written.
func (proxy *ProxyResponseWriter) Written() bool {
	if proxy.code != 0 {
		return true
	}
	return proxy.internal.Written()
}

// Size returns the size of the response body.
func (proxy *ProxyResponseWriter) Size() int {
	return proxy.internal.Size()
}

// Before allows for a function to be called before the ResponseWriter has been written to. This is
// useful for setting headers or any other operations that must happen before a response has been written.
func (proxy *ProxyResponseWriter) Before(before macaron.BeforeFunc) {
	proxy.internal.Before(before)
}

// Write appends data to the proxied gzip writer.
func (proxy *ProxyResponseWriter) Write(b []byte) (int, error) {
	// if writer is initialized, use the writer
	if proxy.writer != nil {
		return proxy.writer.Write(b)
	}

	proxy.buf = append(proxy.buf, b...)

	var (
		contentLength, _ = strconv.Atoi(proxy.Header().Get(contentLengthHeader))
		contentType      = proxy.Header().Get(contentTypeHeader)
		contentEncoding  = proxy.Header().Get(contentEncodingHeader)
	)

	// OK if an encoding hasn't been chosen, and content length > 1400
	// and content type isn't a compressed type
	if contentEncoding == "" &&
		(contentLength == 0 || contentLength >= MinSize) &&
		(contentType == "" || !compressedContentType(contentType)) {
		// If current buffer is less than the min size and a Content-Length isn't set, then wait
		if len(proxy.buf) < MinSize && contentLength == 0 {
			return len(b), nil
		}

		// If the Content-Length is larger than minSize or the current buffer is larger than minSize, then continue.
		if contentLength >= MinSize || len(proxy.buf) >= MinSize {
			// if we don't know the content type, infer it
			if contentType == "" {
				contentType = http.DetectContentType(proxy.buf)
				proxy.Header().Set(contentTypeHeader, contentType)
			}
			// If the Content-Type is not compressed - Compress!
			if !compressedContentType(contentType) {
				if err := proxy.startGzip(); err != nil {
					return 0, err
				}
				return len(b), nil
			}
		}
	}
	// If we got here, we should not GZIP this response.
	if err := proxy.startPlain(); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (proxy *ProxyResponseWriter) startGzip() error {
	// Set the content-encoding and vary headers.
	proxy.Header().Set(contentEncodingHeader, "gzip")
	proxy.Header().Set(varyHeader, acceptEncodingHeader)

	// if the Content-Length is already set, then calls to Write on gzip
	// will fail to set the Content-Length header since its already set
	// See: https://github.com/golang/go/issues/14975.
	proxy.Header().Del(contentLengthHeader)

	// Write the header to gzip response.
	if proxy.code != 0 {
		proxy.internal.WriteHeader(proxy.code)
		// Ensure that no other WriteHeader's happen
		proxy.code = 0
	}

	// Initialize and flush the buffer into the gzip response if there are any bytes.
	// If there aren't any, we shouldn't initialize it yet because on Close it will
	// write the gzip header even if nothing was ever written.
	if len(proxy.buf) > 0 {
		// Initialize the GZIP response.
		proxy.writer = writerPool.Get(proxy.internal)

		return proxy.writeBuf()
	}
	return nil
}

func (proxy *ProxyResponseWriter) startPlain() error {
	if proxy.code != 0 {
		proxy.internal.WriteHeader(proxy.code)
		proxy.code = 0
	}
	proxy.stopped = true
	proxy.writer = noopCloser{proxy.internal}
	return proxy.writeBuf()
}

func (proxy *ProxyResponseWriter) writeBuf() error {
	if proxy.buf == nil {
		return nil
	}

	n, err := proxy.writer.Write(proxy.buf)

	// This should never happen (per io.Writer docs), but if the write didn't
	// accept the entire buffer but returned no specific error, we have no clue
	// what's going on, so abort just to be safe.
	if err == nil && n < len(proxy.buf) {
		err = io.ErrShortWrite
	}
	proxy.buf = nil
	return err
}

// WriteHeader will ensure that we have setup the writer before we write the header
func (proxy *ProxyResponseWriter) WriteHeader(code int) {
	if proxy.code == 0 {
		proxy.code = code
	}
}

// Close the writer
func (proxy *ProxyResponseWriter) Close() error {
	if proxy.stopped {
		return nil
	}

	if proxy.writer == nil {
		err := proxy.startPlain()
		if err != nil {
			return fmt.Errorf("GzipMiddleware: write to regular responseWriter at close gets error: %q", err.Error())
		}
	}

	err := proxy.writer.Close()

	if poolWriter, ok := proxy.writer.(*gzip.Writer); ok {
		writerPool.Put(poolWriter)
	}

	proxy.writer = nil
	proxy.stopped = true
	return err
}

// Flush the writer
func (proxy *ProxyResponseWriter) Flush() {
	if proxy.writer == nil {
		return
	}

	if gw, ok := proxy.writer.(*gzip.Writer); ok {
		gw.Flush()
	}

	proxy.internal.Flush()
}

// Hijack implements http.Hijacker. If the underlying ResponseWriter is a
// Hijacker, its Hijack method is returned. Otherwise an error is returned.
func (proxy *ProxyResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := proxy.internal.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("the ResponseWriter doesn't support the Hijacker interface")
	}
	return hijacker.Hijack()
}

// verify Hijacker interface implementation
var _ http.Hijacker = &ProxyResponseWriter{}

func compressedContentType(contentType string) bool {
	switch contentType {
	case "application/zip":
		return true
	case "application/x-gzip":
		return true
	case "application/gzip":
		return true
	default:
		return false
	}
}
