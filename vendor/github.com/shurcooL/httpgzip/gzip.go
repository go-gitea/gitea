package httpgzip

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"golang.org/x/net/http/httpguts"
)

// GzipByter is implemented by compressed files for
// efficient direct access to the internal compressed bytes.
type GzipByter interface {
	// GzipBytes returns gzip compressed contents of the file.
	GzipBytes() []byte
}

// NotWorthGzipCompressing is implemented by files that were determined
// not to be worth gzip compressing (the file size did not decrease as a result).
type NotWorthGzipCompressing interface {
	// NotWorthGzipCompressing is a noop. It's implemented in order to indicate
	// the file is not worth gzip compressing.
	NotWorthGzipCompressing()
}

// ServeContent is like http.ServeContent, except it applies gzip compression
// if compression hasn't already been done (i.e., the "Content-Encoding" header is set).
// It's aware of GzipByter and NotWorthGzipCompressing interfaces, and uses them
// to improve performance when the provided content implements them. Otherwise,
// it applies gzip compression on the fly, if it's found to be beneficial.
func ServeContent(w http.ResponseWriter, req *http.Request, name string, modTime time.Time, content io.ReadSeeker) {
	// If compression has already been dealt with, serve as is.
	if _, ok := w.Header()["Content-Encoding"]; ok {
		http.ServeContent(w, req, name, modTime, content)
		return
	}

	// If request doesn't accept gzip encoding, serve without compression.
	if !httpguts.HeaderValuesContainsToken(req.Header["Accept-Encoding"], "gzip") {
		http.ServeContent(w, req, name, modTime, content)
		return
	}

	// If the file is not worth gzip compressing, serve it as is.
	if _, ok := content.(NotWorthGzipCompressing); ok {
		w.Header()["Content-Encoding"] = nil
		http.ServeContent(w, req, name, modTime, content)
		return
	}

	// The following cases involve compression, so we want to detect the Content-Type eagerly,
	// before passing it off to http.ServeContent. It's because http.ServeContent won't be able
	// to easily detect the original content type after content has been gzip compressed.
	// We do this even for the last case that serves uncompressed data so that it doesn't
	// have to do duplicate work.
	_, haveType := w.Header()["Content-Type"]
	if !haveType {
		ctype := mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			// Read a chunk to decide between utf-8 text and binary.
			var buf [512]byte
			n, _ := io.ReadFull(content, buf[:])
			ctype = http.DetectContentType(buf[:n])
			_, err := content.Seek(0, io.SeekStart) // Rewind to output whole file.
			if err != nil {
				http.Error(w, "500 Internal Server Error\n\nseeker can't seek", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", ctype)
	}

	// If there are gzip encoded bytes available, use them directly.
	if gzipFile, ok := content.(GzipByter); ok {
		w.Header().Set("Content-Encoding", "gzip")
		http.ServeContent(w, req, name, modTime, bytes.NewReader(gzipFile.GzipBytes()))
		return
	}

	// Perform compression and serve gzip compressed bytes (if it's worth it).
	if rs, err := gzipCompress(content); err == nil {
		w.Header().Set("Content-Encoding", "gzip")
		http.ServeContent(w, req, name, modTime, rs)
		return
	}

	// Serve as is.
	w.Header()["Content-Encoding"] = nil
	http.ServeContent(w, req, name, modTime, content)
}

// gzipCompress compresses input from r and returns it as an io.ReadSeeker.
// It returns an error if compressed size is not smaller than uncompressed.
func gzipCompress(r io.Reader) (io.ReadSeeker, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	n, err := io.Copy(gw, r)
	if err != nil {
		// No need to gw.Close() here since we're discarding the result, and gzip.Writer.Close isn't needed for cleanup.
		return nil, err
	}
	err = gw.Close()
	if err != nil {
		return nil, err
	}
	if int64(buf.Len()) >= n {
		return nil, fmt.Errorf("not worth gzip compressing: original size %v, compressed size %v", n, buf.Len())
	}
	return bytes.NewReader(buf.Bytes()), nil
}
