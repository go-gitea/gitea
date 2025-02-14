// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package uri

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadURI(t *testing.T) {
	p, err := filepath.Abs("./uri.go")
	assert.NoError(t, err)
	f, err := Open("file://" + p)
	assert.NoError(t, err)
	defer f.Close()
}
