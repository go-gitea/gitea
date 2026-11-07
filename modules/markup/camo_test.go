// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package markup

import (
	"testing"

	"gitea.dev/modules/setting"
	testModule "gitea.dev/modules/test"

	"github.com/stretchr/testify/assert"
)

func TestCamoHandleLink(t *testing.T) {
	defer testModule.MockVariableValue(&setting.AppURL, "https://gitea.com")()
	defer testModule.MockVariableValue(&setting.Camo)()
	// Test media proxy
	setting.Camo.Enabled = true
	setting.Camo.Always = false
	setting.Camo.ServerURL = "https://image.proxy"
	setting.Camo.HMACKey = "geheim"
	dataURL := "data:image/png;base64,i//V"
	test := func(expectedHTTPS string) {
		assert.Equal(t, dataURL, camoHandleLink(dataURL))
		assert.Equal(t,
			"https://gitea.com/img.jpg",
			camoHandleLink("https://gitea.com/img.jpg"))
		assert.Equal(t, expectedHTTPS, camoHandleLink("https://testimages.org/img.jpg"))
		assert.Equal(t,
			"https://image.proxy/eivin43gJwGVIjR9MiYYtFIk0mw/aHR0cDovL3Rlc3RpbWFnZXMub3JnL2ltZy5qcGc",
			camoHandleLink("http://testimages.org/img.jpg"))
	}

	test("https://testimages.org/img.jpg")
	setting.Camo.Always = true
	test("https://image.proxy/tkdlvmqpbIr7SjONfHNgEU622y0/aHR0cHM6Ly90ZXN0aW1hZ2VzLm9yZy9pbWcuanBn")
}
