package middleware

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

type encoding int

const (
	encodingNone encoding = iota
	encodingGzip
	encodingDeflate
)

var defaultContentTypes = map[string]struct{}{
	"text/html":                struct{}{},
	"text/css":                 struct{}{},
	"text/plain":               struct{}{},
	"text/javascript":          struct{}{},
	"application/javascript":   struct{}{},
	"application/x-javascript": struct{}{},
	"application/json":         struct{}{},
	"application/atom+xml":     struct{}{},
	"application/rss+xml":      struct{}{},
}

// DefaultCompress is a middleware that compresses response
// body of predefined content types to a data format based
// on Accept-Encoding request header. It uses a default
// compression level.
func DefaultCompress(next http.Handler) http.Handler {
	return Compress(flate.DefaultCompression)(next)
}

// Compress is a middleware that compresses response
// body of a given content types to a data format based
// on Accept-Encoding request header. It uses a given
// compression level.
func Compress(level int, types ...string) func(next http.Handler) http.Handler {
	contentTypes := defaultContentTypes
	if len(types) > 0 {
		contentTypes = make(map[string]struct{}, len(types))
		for _, t := range types {
			contentTypes[t] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			mcw := &maybeCompressResponseWriter{
				ResponseWriter: w,
				w:              w,
				contentTypes:   contentTypes,
				encoding:       selectEncoding(r.Header),
				level:          level,
			}
			defer mcw.Close()

			next.ServeHTTP(mcw, r)
		}

		return http.HandlerFunc(fn)
	}
}

func selectEncoding(h http.Header) encoding {
	enc := h.Get("Accept-Encoding")

	switch {
	// TODO:
	// case "br":    // Brotli, experimental. Firefox 2016, to-be-in Chromium.
	// case "lzma":  // Opera.
	// case "sdch":  // Chrome, Android. Gzip output + dictionary header.

	case strings.Contains(enc, "gzip"):
		// TODO: Exception for old MSIE browsers that can't handle non-HTML?
		// https://zoompf.com/blog/2012/02/lose-the-wait-http-compression
		return encodingGzip

	case strings.Contains(enc, "deflate"):
		// HTTP 1.1 "deflate" (RFC 2616) stands for DEFLATE data (RFC 1951)
		// wrapped with zlib (RFC 1950). The zlib wrapper uses Adler-32
		// checksum compared to CRC-32 used in "gzip" and thus is faster.
		//
		// But.. some old browsers (MSIE, Safari 5.1) incorrectly expect
		// raw DEFLATE data only, without the mentioned zlib wrapper.
		// Because of this major confusion, most modern browsers try it
		// both ways, first looking for zlib headers.
		// Quote by Mark Adler: http://stackoverflow.com/a/9186091/385548
		//
		// The list of browsers having problems is quite big, see:
		// http://zoompf.com/blog/2012/02/lose-the-wait-http-compression
		// https://web.archive.org/web/20120321182910/http://www.vervestudios.co/projects/compression-tests/results
		//
		// That's why we prefer gzip over deflate. It's just more reliable
		// and not significantly slower than gzip.
		return encodingDeflate

		// NOTE: Not implemented, intentionally:
		// case "compress": // LZW. Deprecated.
		// case "bzip2":    // Too slow on-the-fly.
		// case "zopfli":   // Too slow on-the-fly.
		// case "xz":       // Too slow on-the-fly.
	}

	return encodingNone
}

type maybeCompressResponseWriter struct {
	http.ResponseWriter
	w            io.Writer
	encoding     encoding
	contentTypes map[string]struct{}
	level        int
	wroteHeader  bool
}

func (w *maybeCompressResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	defer w.ResponseWriter.WriteHeader(code)

	// Already compressed data?
	if w.ResponseWriter.Header().Get("Content-Encoding") != "" {
		return
	}
	// The content-length after compression is unknown
	w.ResponseWriter.Header().Del("Content-Length")

	// Parse the first part of the Content-Type response header.
	contentType := ""
	parts := strings.Split(w.ResponseWriter.Header().Get("Content-Type"), ";")
	if len(parts) > 0 {
		contentType = parts[0]
	}

	// Is the content type compressable?
	if _, ok := w.contentTypes[contentType]; !ok {
		return
	}

	// Select the compress writer.
	switch w.encoding {
	case encodingGzip:
		gw, err := gzip.NewWriterLevel(w.ResponseWriter, w.level)
		if err != nil {
			w.w = w.ResponseWriter
			return
		}
		w.w = gw
		w.ResponseWriter.Header().Set("Content-Encoding", "gzip")

	case encodingDeflate:
		dw, err := flate.NewWriter(w.ResponseWriter, w.level)
		if err != nil {
			w.w = w.ResponseWriter
			return
		}
		w.w = dw
		w.ResponseWriter.Header().Set("Content-Encoding", "deflate")
	}
}

func (w *maybeCompressResponseWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.w.Write(p)
}

func (w *maybeCompressResponseWriter) Flush() {
	if f, ok := w.w.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *maybeCompressResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := w.w.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("chi/middleware: http.Hijacker is unavailable on the writer")
}

func (w *maybeCompressResponseWriter) CloseNotify() <-chan bool {
	if cn, ok := w.w.(http.CloseNotifier); ok {
		return cn.CloseNotify()
	}

	// If the underlying writer does not implement http.CloseNotifier, return
	// a channel that never receives a value. The semantics here is that the
	// client never disconnnects before the request is processed by the
	// http.Handler, which is close enough to the default behavior (when
	// CloseNotify() is not even called).
	return make(chan bool, 1)
}

func (w *maybeCompressResponseWriter) Close() error {
	if c, ok := w.w.(io.WriteCloser); ok {
		return c.Close()
	}
	return errors.New("chi/middleware: io.WriteCloser is unavailable on the writer")
}
