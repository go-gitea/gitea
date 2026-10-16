// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package helm

import (
	"compress/gzip"
	"strings"
	"testing"

	"gitea.dev/modules/packages"
	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
)

const chartFile = "apiVersion: v2\nname: test\nversion: 1.0.0\n"

func TestParseChartArchive(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		data := test.WriteTarCompression(gzip.NewWriter, map[string]string{"test/Chart.yaml": chartFile})
		metadata, err := ParseChartArchive(data)
		assert.NoError(t, err)
		assert.Equal(t, "test", metadata.Name)
	})

	t.Run("TooLarge", func(t *testing.T) {
		defer test.MockVariableValue(&maxChartArchiveSize, 1024)()
		data := test.WriteTarCompression(gzip.NewWriter, map[string]string{"test/padding": strings.Repeat("a", 2048)})
		metadata, err := ParseChartArchive(data)
		assert.Nil(t, metadata)
		assert.ErrorIs(t, err, util.ErrLimitExceeded)
	})
}

func TestParseChartFile(t *testing.T) {
	metadata, err := ParseChartFile(strings.NewReader(chartFile + "description: " + strings.Repeat("a", packages.MaxMetadataFileSize) + "\n"))
	assert.Nil(t, metadata)
	assert.ErrorIs(t, err, util.ErrInvalidArgument)
}
