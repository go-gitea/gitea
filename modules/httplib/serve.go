// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	charsetModule "code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/container"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"

	"github.com/klauspost/compress/gzhttp"
)

type ServeHeaderOptions struct {
	ContentType   string // defaults to "application/octet-stream"
	ContentLength *int64

	Filename           string
	ContentDisposition ContentDispositionType

	CacheIsPublic bool
	CacheDuration time.Duration // defaults to 5 minutes
	LastModified  time.Time
}

// ServeSetHeaders sets necessary content serve headers
func ServeSetHeaders(w http.ResponseWriter, opts ServeHeaderOptions) {
	header := w.Header()

	skipCompressionExts := container.SetOf(".gz", ".bz2", ".zip", ".xz", ".zst", ".deb", ".apk", ".jar", ".png", ".jpg", ".webp")
	if skipCompressionExts.Contains(strings.ToLower(path.Ext(opts.Filename))) {
		w.Header().Add(gzhttp.HeaderNoCompression, "1")
	}

	contentType := util.IfZero(opts.ContentType, typesniffer.MimeTypeApplicationOctetStream)
	header.Set("Content-Type", contentType)
	header.Set("X-Content-Type-Options", "nosniff")

	if opts.ContentLength != nil {
		header.Set("Content-Length", strconv.FormatInt(*opts.ContentLength, 10))
	}

	// Disable script execution of HTML/SVG files, since we serve the file from the same origin as Gitea server
	header.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	if strings.Contains(contentType, "application/pdf") {
		// no sandbox attribute for PDF as it breaks rendering in at least safari. this
		// should generally be safe as scripts inside PDF can not escape the PDF document
		// see https://bugs.chromium.org/p/chromium/issues/detail?id=413851 for more discussion
		// HINT: PDF-RENDER-SANDBOX: PDF won't render in sandboxed context
		header.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	}

	if opts.Filename != "" && opts.ContentDisposition != "" {
		header.Set("Content-Disposition", encodeContentDisposition(opts.ContentDisposition, path.Base(opts.Filename)))
		header.Set("Access-Control-Expose-Headers", "Content-Disposition")
	}

	httpcache.SetCacheControlInHeader(header, &httpcache.CacheControlOptions{
		IsPublic:    opts.CacheIsPublic,
		MaxAge:      opts.CacheDuration,
		NoTransform: true,
	})

	if !opts.LastModified.IsZero() {
		// http.TimeFormat required a UTC time, refer to https://pkg.go.dev/net/http#TimeFormat
		header.Set("Last-Modified", opts.LastModified.UTC().Format(http.TimeFormat))
	}
}

func serveSetHeadersByUserContent(w http.ResponseWriter, contentPrefetchBuf []byte, opts ServeHeaderOptions) {
	var detectCharset bool

	if setting.MimeTypeMap.Enabled {
		fileExtension := strings.ToLower(path.Ext(opts.Filename))
		opts.ContentType = setting.MimeTypeMap.Map[fileExtension]
		detectCharset = strings.HasPrefix(opts.ContentType, "text/") && !strings.Contains(opts.ContentType, "charset=")
	}

	if opts.ContentType == "" {
		sniffedType := typesniffer.DetectContentType(contentPrefetchBuf)
		if sniffedType.IsBrowsableBinaryType() {
			opts.ContentType = sniffedType.GetMimeType()
		} else if sniffedType.IsText() {
			//  intentionally do not render user's HTML content as a page, for safety, and avoid content spamming & abusing
			opts.ContentType = "text/plain"
			detectCharset = true
		} else {
			opts.ContentType = typesniffer.MimeTypeApplicationOctetStream
		}
	}

	if detectCharset {
		if charset, _ := charsetModule.DetectEncoding(contentPrefetchBuf); charset != "" {
			opts.ContentType += "; charset=" + strings.ToLower(charset)
		}
	}

	if opts.ContentDisposition == "" {
		sniffedType := typesniffer.FromContentType(opts.ContentType)
		opts.ContentDisposition = ContentDispositionInline
		if sniffedType.IsSvgImage() && !setting.UI.SVG.Enabled {
			opts.ContentDisposition = ContentDispositionAttachment
		}
	}

	ServeSetHeaders(w, opts)
}

const mimeDetectionBufferLen = 1024

func ServeUserContentByReader(r *http.Request, w http.ResponseWriter, size int64, reader io.Reader, opts ServeHeaderOptions) {
	if opts.ContentLength != nil {
		panic("do not set ContentLength, use size argument instead")
	}
	buf := make([]byte, mimeDetectionBufferLen)
	n, err := util.ReadAtMost(reader, buf)
	if err != nil {
		http.Error(w, "serve content: unable to pre-read", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if n >= 0 {
		buf = buf[:n]
	}
	serveSetHeadersByUserContent(w, buf, opts)

	// reset the reader to the beginning
	reader = io.MultiReader(bytes.NewReader(buf), reader)

	rangeHeader := r.Header.Get("Range")

	// if no size or no supported range, serve as 200 (complete response)
	if size <= 0 || !strings.HasPrefix(rangeHeader, "bytes=") {
		if size >= 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		}
		_, _ = io.Copy(w, reader) // just like http.ServeContent, not necessary to handle the error
		return
	}

	// do our best to support the minimal "Range" request (no support for multiple range: "Range: bytes=0-50, 100-150")
	//
	// GET /...
	// Range: bytes=0-1023
	//
	// HTTP/1.1 206 Partial Content
	// Content-Range: bytes 0-1023/146515
	// Content-Length: 1024

	_, rangeParts, _ := strings.Cut(rangeHeader, "=")
	rangeBytesStart, rangeBytesEnd, found := strings.Cut(rangeParts, "-")
	start, err := strconv.ParseInt(rangeBytesStart, 10, 64)
	if start < 0 || start >= size {
		err = errors.New("invalid start range")
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
		return
	}
	end, err := strconv.ParseInt(rangeBytesEnd, 10, 64)
	if rangeBytesEnd == "" && found {
		err = nil
		end = size - 1
	}
	if end >= size {
		end = size - 1
	}
	if end < start {
		err = errors.New("invalid end range")
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	partialLength := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", strconv.FormatInt(partialLength, 10))

	if seeker, ok := reader.(io.Seeker); ok {
		if _, err = seeker.Seek(start, io.SeekStart); err != nil {
			http.Error(w, "serve content: unable to seek", http.StatusInternalServerError)
			return
		}
	} else {
		if _, err = io.CopyN(io.Discard, reader, start); err != nil {
			http.Error(w, "serve content: unable to skip", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusPartialContent)
	_, _ = io.CopyN(w, reader, partialLength) // just like http.ServeContent, not necessary to handle the error
}

func ServeUserContentByFile(r *http.Request, w http.ResponseWriter, file fs.File, opts ServeHeaderOptions) {
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "unable to serve file, stat error", http.StatusInternalServerError)
		return
	}
	opts.LastModified = info.ModTime()
	ServeUserContentByReader(r, w, info.Size(), file, opts)
}
