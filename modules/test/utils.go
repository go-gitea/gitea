// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"gitea.dev/modules/json"
	"gitea.dev/modules/util"

	"golang.org/x/net/html"
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

func ParseJSONRedirect(buf []byte) (ret struct {
	Redirect *string `json:"redirect"`
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

func WriteZipArchive(files map[string]string) *bytes.Buffer {
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()
	return buf
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

var AllowSkipExternalService = sync.OnceValue(func() bool {
	isLocalTesting := os.Getenv("CI") == ""
	ciSkipExternal, _ := strconv.ParseBool(os.Getenv("GITEA_TEST_CI_SKIP_EXTERNAL"))
	return isLocalTesting || ciSkipExternal
})

type TestingT interface {
	Helper()
	Skipf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

func ExternalServiceHTTP(t TestingT, envVarName, def string) string {
	t.Helper()
	val := util.IfZero(os.Getenv(envVarName), def)
	if val == "" {
		if AllowSkipExternalService() {
			t.Skipf("skipping test because %s is not set", envVarName)
		} else {
			t.Fatalf("%s is not set, but skipping is not allowed in CI", envVarName)
		}
	}
	// minio's endpoint is "host:port" pattern
	testURL := util.Iif(strings.Contains(val, "://"), val, "http://"+val)
	resp, err := http.Get(testURL)
	if err != nil {
		if AllowSkipExternalService() {
			t.Skipf("skipping test because %s is not ready", val)
		} else {
			t.Fatalf("%s is not ready, but skipping is not allowed in CI", val)
		}
	} else {
		_ = resp.Body.Close()
	}
	return val
}

var normalizeHTMLSpacesRegexp = sync.OnceValue(func() (ret struct {
	afterRt, beforeLt *regexp.Regexp
},
) {
	ret.afterRt = regexp.MustCompile(`>\s*`)
	ret.beforeLt = regexp.MustCompile(`\s*<`)
	return ret
})

func NormalizeHTMLSpaces(s string) string {
	vars := normalizeHTMLSpacesRegexp()
	s = vars.afterRt.ReplaceAllString(s, ">\n")
	s = vars.beforeLt.ReplaceAllString(s, "\n<")
	return strings.TrimSpace(s)
}

func NormalizeHTMLAttributes(t TestingT, s string) string {
	nodes, err := html.Parse(strings.NewReader(s))
	if err != nil {
		t.Errorf("failed to parse expected HTML: %v", err)
		return ""
	}

	var normalize func(n *html.Node)
	normalize = func(n *html.Node) {
		slices.SortFunc(n.Attr, func(a, b html.Attribute) int {
			if cmp := strings.Compare(a.Namespace, b.Namespace); cmp != 0 {
				return cmp
			}
			if cmp := strings.Compare(a.Key, b.Key); cmp != 0 {
				return cmp
			}
			return strings.Compare(a.Val, b.Val)
		})
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			normalize(c)
		}
	}
	var sb strings.Builder
	if err = html.Render(&sb, nodes); err != nil {
		t.Errorf("failed to render HTML: %v", err)
	}
	return sb.String()
}
