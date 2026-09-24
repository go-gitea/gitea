// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitea.dev/modules/public"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/typesniffer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapArtifactPreviewPaths(t *testing.T) {
	paths := make([]string, artifactPreviewMaxFiles+10)
	for i := range paths {
		paths[i] = "file-" + strconv.Itoa(i) + ".txt"
	}

	capped, truncated := capArtifactPreviewPaths(paths)
	require.True(t, truncated)
	require.Len(t, capped, artifactPreviewMaxFiles)
	paths[0] = "changed"
	assert.Equal(t, "file-0.txt", capped[0])

	short := []string{"a.txt", "b.txt"}
	capped, truncated = capArtifactPreviewPaths(short)
	assert.False(t, truncated)
	assert.Equal(t, short, capped)
}

func TestInsertArtifactPreviewPath(t *testing.T) {
	paths := []string{"a.txt", "c.txt", "dir/a.txt"}

	assert.Equal(t, []string{"a.txt", "b.txt", "c.txt", "dir/a.txt"}, insertArtifactPreviewPath(paths, "b.txt"))
	assert.Equal(t, []string{"a.txt", "c.txt", "dir/a.txt"}, paths)
	assert.Equal(t, []string{"a.txt", "c.txt", "dir/a.txt", "z.txt"}, insertArtifactPreviewPath(paths, "z.txt"))
}

func TestBuildArtifactPreviewFiles(t *testing.T) {
	files := BuildArtifactPreviewFiles([]string{
		"README.md",
		"report/assets/chart.svg",
		"report/index.html",
		"report/style.css",
	}, "report/index.html")

	assert.Equal(t, []ArtifactPreviewFile{
		{Path: "README.md", Name: "README.md"},
		{Path: "report", Name: "report", IsDir: true},
		{Path: "report/assets", Name: "assets", Depth: 1, IsDir: true},
		{Path: "report/assets/chart.svg", Name: "chart.svg", Depth: 2},
		{Path: "report/index.html", Name: "index.html", Depth: 1, Selected: true},
		{Path: "report/style.css", Name: "style.css", Depth: 1},
	}, files)
}

func TestArtifactPreviewContentTypeUsesPreviewableExtensions(t *testing.T) {
	sniffedText := typesniffer.FromContentType("text/plain; charset=utf-8")

	assert.Equal(t, "text/html; charset=utf-8", artifactPreviewContentType("index.html", sniffedText))
	assert.Equal(t, "text/html; charset=utf-8", artifactPreviewContentType("index.htm", sniffedText))
	assert.Equal(t, "text/css; charset=utf-8", artifactPreviewContentType("style.css", sniffedText))
	assert.Equal(t, "text/javascript; charset=utf-8", artifactPreviewContentType("script.js", sniffedText))
	assert.Equal(t, "text/plain; charset=utf-8", artifactPreviewContentType("output.txt", sniffedText))
	assert.True(t, isPreviewableArtifactType(typesniffer.FromContentType("image/svg+xml")))
}

func TestArtifactPreviewHTMLReader(t *testing.T) {
	const content = "<html>report</html>"
	reader := artifactPreviewHTMLReader(strings.NewReader(content), int64(len(content)), `a="&b=1`)
	prefix := `<script crossorigin src="` + public.AssetURI("web_src/js/external-render-helper.ts") + `" id="gitea-external-render-helper" data-render-query-string="a=&#34;&amp;b=1"></script>`

	result, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, prefix+content, string(result))

	_, err = reader.Seek(int64(len(prefix)-1), io.SeekStart)
	require.NoError(t, err)
	result, err = io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, ">"+content, string(result))

	request := httptest.NewRequest(http.MethodGet, "/preview/raw/index.html", nil)
	request.Header.Set("Range", "bytes="+strconv.Itoa(len(prefix)-1)+"-"+strconv.Itoa(len(prefix)+1))
	response := httptest.NewRecorder()
	http.ServeContent(response, request, "index.html", time.Time{}, artifactPreviewHTMLReader(strings.NewReader(content), int64(len(content)), `a="&b=1`))
	assert.Equal(t, http.StatusPartialContent, response.Code)
	assert.Equal(t, ">"+content[:2], response.Body.String())
}

func TestArtifactPreviewMaxSize(t *testing.T) {
	for _, testCase := range []struct {
		maxSize int64
		size    int64
		allowed bool
	}{
		{maxSize: -1, size: 1 << 30, allowed: true},
		{maxSize: 0, size: 0, allowed: false},
		{maxSize: 10, size: 10, allowed: true},
		{maxSize: 10, size: 11, allowed: false},
	} {
		t.Run(strconv.FormatInt(testCase.maxSize, 10), func(t *testing.T) {
			defer test.MockVariableValue(&setting.Actions.ArtifactPreviewMaxSize, testCase.maxSize)()
			assert.Equal(t, testCase.allowed, isArtifactPreviewSizeAllowed(testCase.size))
		})
	}
}
