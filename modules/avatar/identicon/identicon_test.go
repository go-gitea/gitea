// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build test_avatar_identicon

package identicon

import (
	"image/color"
	"image/png"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	dir, _ := os.Getwd()
	dir = dir + "/testdata"
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Errorf("can not save generated images to %s", dir)
	}

	backColor := color.White
	imgMaker, err := New(64, backColor, DarkColors...)
	assert.NoError(t, err)
	for i := 0; i < 100; i++ {
		s := strconv.Itoa(i)
		img := imgMaker.Make([]byte(s))

		f, err := os.Create(dir + "/" + s + ".png")
		if !assert.NoError(t, err) {
			continue
		}
		defer f.Close()
		err = png.Encode(f, img)
		assert.NoError(t, err)
	}
}
