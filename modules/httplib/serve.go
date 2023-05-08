// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	charsetModule "code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
)

type ServeHeaderOptions struct {
	ContentType        string // defaults to "application/octet-stream"
	ContentTypeCharset string
	ContentLength      *int64
	Disposition        string // defaults to "attachment"
	Filename           string
	CacheDuration      time.Duration // defaults to 5 minutes
	LastModified       time.Time
}

// ServeSetHeaders sets necessary content serve headers
func ServeSetHeaders(w http.ResponseWriter, opts *ServeHeaderOptions) {
	header := w.Header()

	contentType := typesniffer.ApplicationOctetStream
	if opts.ContentType != "" {
		if opts.ContentTypeCharset != "" {
			contentType = opts.ContentType + "; charset=" + strings.ToLower(opts.ContentTypeCharset)
		} else {
			contentType = opts.ContentType
		}
	}
	header.Set("Content-Type", contentType)
	header.Set("X-Content-Type-Options", "nosniff")

	if opts.ContentLength != nil {
		header.Set("Content-Length", strconv.FormatInt(*opts.ContentLength, 10))
	}

	if opts.Filename != "" {
		disposition := opts.Disposition
		if disposition == "" {
			disposition = "attachment"
		}

		backslashEscapedName := strings.ReplaceAll(strings.ReplaceAll(opts.Filename, `\`, `\\`), `"`, `\"`) // \ -> \\, " -> \"
		header.Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"; filename*=UTF-8''%s`, disposition, backslashEscapedName, url.PathEscape(opts.Filename)))
		header.Set("Access-Control-Expose-Headers", "Content-Disposition")
	}

	duration := opts.CacheDuration
	if duration == 0 {
		duration = 5 * time.Minute
	}
	httpcache.SetCacheControlInHeader(header, duration)

	if !opts.LastModified.IsZero() {
		header.Set("Last-Modified", opts.LastModified.UTC().Format(http.TimeFormat))
	}
}

// ServeData download file from io.Reader
func setServeHeadersByFile(r *http.Request, w http.ResponseWriter, filePath string, mineBuf []byte) error {
	// do not set "Content-Length", because the length could only be set by callers, and it needs to support range requests
	opts := &ServeHeaderOptions{
		Filename: path.Base(filePath),
	}

	sniffedType := typesniffer.DetectContentType(mineBuf)

	// the "render" parameter came from year 2016: 638dd24c, it doesn't have clear meaning, so I think it could be removed later
	isPlain := sniffedType.IsText() || r.FormValue("render") != ""

	if setting.MimeTypeMap.Enabled {
		fileExtension := strings.ToLower(filepath.Ext(filePath))
		opts.ContentType = setting.MimeTypeMap.Map[fileExtension]
	}

	if opts.ContentType == "" {
		if sniffedType.IsBrowsableBinaryType() {
			opts.ContentType = sniffedType.GetMimeType()
		} else if isPlain {
			opts.ContentType = "text/plain"
		} else {
			opts.ContentType = typesniffer.ApplicationOctetStream
		}
	}

	if isPlain {
		charset, err := charsetModule.DetectEncoding(mineBuf)
		if err != nil {
			log.Error("Detect raw file %s charset failed: %v, using by default utf-8", filePath, err)
			charset = "utf-8"
		}
		opts.ContentTypeCharset = strings.ToLower(charset)
	}

	isSVG := sniffedType.IsSvgImage()

	// serve types that can present a security risk with CSP
	if isSVG {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	} else if sniffedType.IsPDF() {
		// no sandbox attribute for pdf as it breaks rendering in at least safari. this
		// should generally be safe as scripts inside PDF can not escape the PDF document
		// see https://bugs.chromium.org/p/chromium/issues/detail?id=413851 for more discussion
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	}

	opts.Disposition = "inline"
	if isSVG && !setting.UI.SVG.Enabled {
		opts.Disposition = "attachment"
	}

	ServeSetHeaders(w, opts)
	return nil
}

const mimeDetectionBufferLen = 1024

func ServeContentByReader(r *http.Request, w http.ResponseWriter, filePath string, size int64, reader io.Reader) error {
	buf := make([]byte, mimeDetectionBufferLen)
	n, err := util.ReadAtMost(reader, buf)
	if err != nil {
		return err
	}
	if n >= 0 {
		buf = buf[:n]
	}
	if err = setServeHeadersByFile(r, w, filePath, buf); err != nil {
		return err
	}

	// reset the reader to the beginning
	reader = io.MultiReader(bytes.NewReader(buf), reader)

	rangeHeader := r.Header.Get("Range")

	// if no size or no supported range, serve as 200 (complete response)
	if size <= 0 || !strings.HasPrefix(rangeHeader, "bytes=") {
		if size >= 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
		}
		_, err = io.Copy(w, reader)
		return err
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
		return err
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
		return err
	}

	partialLength := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", strconv.FormatInt(partialLength, 10))
	if _, err = io.CopyN(io.Discard, reader, start); err != nil {
		return fmt.Errorf("unable to skip first %d bytes: %w", start, err)
	}

	w.WriteHeader(http.StatusPartialContent)
	_, err = io.CopyN(w, reader, partialLength)
	return err
}

func ServeContentByReadSeeker(r *http.Request, w http.ResponseWriter, filePath string, modTime time.Time, reader io.ReadSeeker) error {
	buf := make([]byte, mimeDetectionBufferLen)
	n, err := util.ReadAtMost(reader, buf)
	if err != nil {
		return err
	}
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if n >= 0 {
		buf = buf[:n]
	}
	if err = setServeHeadersByFile(r, w, filePath, buf); err != nil {
		return err
	}
	http.ServeContent(w, r, path.Base(filePath), modTime, reader)
	return nil
}
