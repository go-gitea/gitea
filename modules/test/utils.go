// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"code.gitea.io/gitea/modules/json"
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
