// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/url"
	"testing"

	"code.gitea.io/gitea/modules/json"

	"github.com/stretchr/testify/assert"
)

func TestMakeAbsoluteAssetURL(t *testing.T) {
	appURL1, _ := url.Parse("https://localhost:1234")
	appURL2, _ := url.Parse("https://localhost:1234/")
	appURLSub1, _ := url.Parse("https://localhost:1234/foo")
	appURLSub2, _ := url.Parse("https://localhost:1234/foo/")

	// static URL is an absolute URL, so should be used
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL(appURL1, "https://localhost:2345"))
	assert.Equal(t, "https://localhost:2345", MakeAbsoluteAssetURL(appURL1, "https://localhost:2345/"))

	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL(appURL1, "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL(appURL2, "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL(appURL1, "/foo/"))

	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL(appURLSub1, "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL(appURLSub2, "/foo"))
	assert.Equal(t, "https://localhost:1234/foo", MakeAbsoluteAssetURL(appURLSub1, "/foo/"))

	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL(appURLSub1, "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL(appURLSub2, "/bar"))
	assert.Equal(t, "https://localhost:1234/bar", MakeAbsoluteAssetURL(appURLSub1, "/bar/"))
}

func TestMakeManifestData(t *testing.T) {
	jsonBytes := MakeManifestData(`Example App '\"`, "https://example.com", "https://example.com/foo/bar")
	assert.True(t, json.Valid(jsonBytes))
}
