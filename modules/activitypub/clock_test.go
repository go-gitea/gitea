// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"regexp"
	"testing"
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestClock(t *testing.T) {
	DefaultUILocation := setting.DefaultUILocation
	defer func() {
		setting.DefaultUILocation = DefaultUILocation
	}()
	c, err := NewClock()
	assert.NoError(t, err)
	setting.DefaultUILocation, err = time.LoadLocation("UTC")
	assert.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`\+0000$`), c.Now().Format(time.Layout))
	setting.DefaultUILocation, err = time.LoadLocation("Europe/Paris")
	assert.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile(`\+0[21]00$`), c.Now().Format(time.Layout))
}
