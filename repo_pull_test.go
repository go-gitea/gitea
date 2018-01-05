// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFormatPatch(t *testing.T) {
	repo, err := OpenRepository(".");
	assert.NoError(t, err)
	patchb, err := repo.GetFormatPatch("cdb43f0e^", "cdb43f0e")
	assert.NoError(t, err)
	patch := string(patchb)
	assert.Regexp(t, "^From cdb43f0e", patch)
	assert.Regexp(t, "Subject: .PATCH. add @daviian as maintainer", patch)
}
