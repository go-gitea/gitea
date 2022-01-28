// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"code.gitea.io/gitea/modules/charset"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/httpcache"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/typesniffer"
	"code.gitea.io/gitea/modules/util"
)

// ServeBlob download a git.Blob
func ServeBlob(ctx *context.Context, blob *git.Blob) error {
	if httpcache.HandleGenericETagCache(ctx.Req, ctx.Resp, `"`+blob.ID.String()+`"`) {
		return nil
	}

	dataRc, err := blob.DataAsync()
	if err != nil {
		return err
	}
	defer func() {
		if err = dataRc.Close(); err != nil {
			log.Error("ServeBlob: Close: %v", err)
		}
	}()

	return ServeData(ctx, ctx.Repo.TreePath, blob.Size(), dataRc)
}

// ServeData download file from io.Reader
func ServeData(ctx *context.Context, name string, size int64, reader io.Reader) error {
	if seeker, ok := reader.(io.Seeker); ok {
		if Range := ctx.Req.Header.Get("Range"); len(Range) > 0 {
			var ranges [][3]int64
			Range = strings.ReplaceAll(strings.TrimLeft(Range, "bytes="), " ", "")
			for _, str := range strings.Split(Range, ",") {
				var rangeStart int64
				var rangeEnd int64
				var length int64
				var err1, err2 error
				slc := strings.Split(str, "-")
				if len(slc) != 2 {
					err1 = http.ErrNotSupported
				} else if len(slc[0]) == 0 {
					length, err1 = strconv.ParseInt(slc[1], 10, 64)
					rangeStart = size - length
					rangeEnd = size - 1
				} else if len(slc[1]) == 0 {
					rangeStart, err1 = strconv.ParseInt(slc[0], 10, 64)
					rangeEnd = size - 1
					length = rangeEnd - rangeStart + 1
				} else {
					rangeStart, err1 = strconv.ParseInt(slc[0], 10, 64)
					rangeEnd, err2 = strconv.ParseInt(slc[1], 10, 64)
					length = rangeEnd - rangeStart + 1
				}

				if nil != err1 || nil != err2 {
					ctx.Resp.WriteHeader(http.StatusBadRequest)
					_, err := ctx.Resp.Write([]byte(fmt.Sprintln("invalid range:", Range)))
					return err
				}

				if rangeStart < 0 || rangeEnd >= size || length <= 0 {
					ctx.Status(http.StatusRequestedRangeNotSatisfiable)
					return nil
				}

				ranges = append(ranges, [3]int64{rangeStart, rangeEnd, length})
			}

			if len(ranges) == 1 {				
				fileExtension := strings.ToLower(filepath.Ext(name))				
				if mappedMimeType, ok := setting.MimeTypeMap.Map[fileExtension]; ok {
					ctx.Resp.Header().Set("Content-Type", mappedMimeType)
				}
				ctx.Resp.Header().Set("Content-Length", strconv.FormatInt(ranges[0][2], 10))
				// ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Range")
				ctx.Resp.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", ranges[0][0], ranges[0][1], size))

				ctx.Resp.WriteHeader(http.StatusPartialContent)

				if ranges[0][0] > 0 {
					if _, err := seeker.Seek(ranges[0][0], io.SeekStart); nil != err {
						return err
					}
				}

				if "HEAD" == ctx.Req.Method {
					return nil
				}

				_, err := io.CopyN(ctx.Resp, reader, ranges[0][2])
				if nil == err || strings.Contains(err.Error(), "write: broken pipe") {
					return nil
				}
				return err
			} else {
				// todo Multipart ranges support
			}
		} else {
			ctx.Resp.Header().Set("Accept-Ranges", "bytes")
		}
	}

	
	buf := make([]byte, 1024)
	n, err := util.ReadAtMost(reader, buf)
	if err != nil {
		return err
	}
	if n >= 0 {
		buf = buf[:n]
	}

	ctx.Resp.Header().Set("Cache-Control", "public,max-age=86400")

	if size >= 0 {
		ctx.Resp.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	} else {
		log.Error("ServeData called to serve data: %s with size < 0: %d", name, size)
	}
	name = path.Base(name)

	// Google Chrome dislike commas in filenames, so let's change it to a space
	name = strings.ReplaceAll(name, ",", " ")

	st := typesniffer.DetectContentType(buf)

	mappedMimeType := ""
	if setting.MimeTypeMap.Enabled {
		fileExtension := strings.ToLower(filepath.Ext(name))
		mappedMimeType = setting.MimeTypeMap.Map[fileExtension]
	}
	if st.IsText() || ctx.FormBool("render") {
		cs, err := charset.DetectEncoding(buf)
		if err != nil {
			log.Error("Detect raw file %s charset failed: %v, using by default utf-8", name, err)
			cs = "utf-8"
		}
		if mappedMimeType == "" {
			mappedMimeType = "text/plain"
		}
		ctx.Resp.Header().Set("Content-Type", mappedMimeType+"; charset="+strings.ToLower(cs))
	} else {
		ctx.Resp.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")
		if mappedMimeType != "" {
			ctx.Resp.Header().Set("Content-Type", mappedMimeType)
		}
		if (st.IsImage() || st.IsPDF()) && (setting.UI.SVG.Enabled || !st.IsSvgImage()) {
			ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, name))
			if st.IsSvgImage() {
				ctx.Resp.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
				ctx.Resp.Header().Set("X-Content-Type-Options", "nosniff")
				ctx.Resp.Header().Set("Content-Type", typesniffer.SvgMimeType)
			}
		} else {
			ctx.Resp.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		}
	}

	_, err = ctx.Resp.Write(buf)
	if err != nil {
		return err
	}
	_, err = io.Copy(ctx.Resp, reader)
	return err
}
