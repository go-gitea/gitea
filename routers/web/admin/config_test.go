// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package admin

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestValidateDiffHighlightCacheConfig(t *testing.T) {
	key := setting.Config().Repository.DiffHighlightCache.DynKey()
	for _, value := range []string{`{"Enabled":true,"SizeMiB":64}`, `{"Enabled":false,"SizeMiB":128}`} {
		assert.NoError(t, validateConfigKeyValue(key, value))
	}
	for _, value := range []string{`{"SizeMiB":0}`, `{"SizeMiB":-1}`, `{"SizeMiB":1.5}`, `{"SizeMiB":"64"}`, `{"Enabled":"true"}`, `{"SizeMiB":9223372036854775807}`} {
		assert.Error(t, validateConfigKeyValue(key, value))
	}
}
