// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

// RedirectURL returns the redirect URL of a http response.
// It also works for JSONRedirect: `{"redirect": "..."}`
// FIXME: it should separate the logic of checking from header and JSON body
func RedirectURL(resp http.ResponseWriter) string {
	loc := resp.Header().Get("Location")
	if loc != "" {
		return loc
	}
	if r, ok := resp.(*httptest.ResponseRecorder); ok {
		m := map[string]any{}
		err := json.Unmarshal(r.Body.Bytes(), &m)
		if err == nil {
			if loc, ok := m["redirect"].(string); ok {
				return loc
			}
		}
	}
	return ""
}

func ParseJSONError(buf []byte) (ret struct {
	ErrorMessage string `json:"errorMessage"`
	RenderFormat string `json:"renderFormat"`
},
) {
	_ = json.Unmarshal(buf, &ret)
	return ret
}

func IsNormalPageCompleted(s string) bool {
	return strings.Contains(s, `<footer class="page-footer"`) && strings.Contains(s, `</html>`)
}

func MockVariableValue[T any](p *T, v ...T) (reset func()) {
	old := *p
	if len(v) > 0 {
		*p = v[0]
	}
	return func() { *p = old }
}

func ReadAllTarGzContent(r io.Reader) (map[string]string, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	content := make(map[string]string)

	tr := tar.NewReader(gzr)
	for {
		hd, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		buf, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}

		content[hd.Name] = string(buf)
	}
	return content, nil
}

func WriteTarArchive(files map[string]string) *bytes.Buffer {
	return WriteTarCompression(func(w io.Writer) io.WriteCloser { return util.NopCloser{Writer: w} }, files)
}

func WriteTarCompression[F func(io.Writer) io.WriteCloser | func(io.Writer) (io.WriteCloser, error)](compression F, files map[string]string) *bytes.Buffer {
	buf := &bytes.Buffer{}
	var cw io.WriteCloser
	switch compressFunc := any(compression).(type) {
	case func(io.Writer) io.WriteCloser:
		cw = compressFunc(buf)
	case func(io.Writer) (io.WriteCloser, error):
		cw, _ = compressFunc(buf)
	}
	tw := tar.NewWriter(cw)

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o600,
			Size: int64(len(content)),
		}
		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write([]byte(content))
	}
	_ = tw.Close()
	_ = cw.Close()
	return buf
}

func CompressGzip(content string) *bytes.Buffer {
	buf := &bytes.Buffer{}
	cw := gzip.NewWriter(buf)
	_, _ = cw.Write([]byte(content))
	_ = cw.Close()
	return buf
}
