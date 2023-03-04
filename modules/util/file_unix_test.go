// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build !windows

package util

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyUmask(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "test-filemode-")
	assert.NoError(t, err)

	err = os.Chmod(f.Name(), 0o777)
	assert.NoError(t, err)
	st, err := os.Stat(f.Name())
	assert.NoError(t, err)
	assert.EqualValues(t, 0o777, st.Mode().Perm()&0o777)

	oldDefaultUmask := defaultUmask
	defaultUmask = 0o037
	defer func() {
		defaultUmask = oldDefaultUmask
	}()
	err = ApplyUmask(f.Name(), os.ModePerm)
	assert.NoError(t, err)
	st, err = os.Stat(f.Name())
	assert.NoError(t, err)
	assert.EqualValues(t, 0o740, st.Mode().Perm()&0o777)
}
