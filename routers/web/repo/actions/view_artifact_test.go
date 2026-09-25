// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"strconv"
	"testing"

	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/typesniffer"

	"github.com/stretchr/testify/assert"
)

func TestNewArtifactPreviewList(t *testing.T) {
	paths := []string{"b.txt", "a.txt", "b.txt"}
	assert.Equal(t, artifactPreviewList{paths: []string{"a.txt", "b.txt"}}, newArtifactPreviewList(paths))

	paths = make([]string, artifactPreviewMaxFiles+10)
	for i := range paths {
		paths[i] = "file-" + strconv.Itoa(i)
	}
	list := newArtifactPreviewList(paths)
	assert.True(t, list.truncated)
	assert.Len(t, list.paths, artifactPreviewMaxFiles)
	paths[0] = "changed"
	assert.Equal(t, "file-0", list.paths[0])
}

func TestBuildArtifactPreviewFiles(t *testing.T) {
	files := buildArtifactPreviewFiles([]string{"README.md", "report/assets/chart.svg", "report/index.html"}, "report/index.html", "/preview/")
	assert.Equal(t, []ArtifactPreviewFile{
		{Path: "README.md", Name: "README.md", Link: "/preview/README.md"},
		{Path: "report", Name: "report"},
		{Path: "report/assets", Name: "assets", Depth: 1},
		{Path: "report/assets/chart.svg", Name: "chart.svg", Link: "/preview/report/assets/chart.svg", Depth: 2},
		{Path: "report/index.html", Name: "index.html", Link: "/preview/report/index.html", Depth: 1, Selected: true},
	}, files)
}

func TestArtifactPreviewContentType(t *testing.T) {
	sniffedText := typesniffer.FromContentType("text/plain; charset=utf-8")
	assert.Equal(t, "text/html; charset=utf-8", artifactPreviewContentType("index.HTM", sniffedText))
	assert.Equal(t, "text/css; charset=utf-8", artifactPreviewContentType("style.css", sniffedText))
	assert.Equal(t, "text/javascript; charset=utf-8", artifactPreviewContentType("script.mjs", sniffedText))
	assert.Equal(t, "text/plain; charset=utf-8", artifactPreviewContentType("output.txt", sniffedText))
	assert.Equal(t, "image/png", artifactPreviewContentType("image.txt", typesniffer.FromContentType("image/png")))
}

func TestIsArtifactPreviewSizeAllowed(t *testing.T) {
	defer test.MockVariableValue(&setting.Actions.ArtifactPreviewMaxSize, int64(-1))()
	assert.True(t, isArtifactPreviewSizeAllowed(1<<40))

	setting.Actions.ArtifactPreviewMaxSize = 0
	assert.False(t, isArtifactPreviewSizeAllowed(0))

	setting.Actions.ArtifactPreviewMaxSize = 10
	assert.True(t, isArtifactPreviewSizeAllowed(10))
	assert.False(t, isArtifactPreviewSizeAllowed(11))
}
