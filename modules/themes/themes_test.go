// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package themes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThemes(t *testing.T) {
	Init()
	assert.Contains(t, Themes, "gitea")
}
