// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeygen(t *testing.T) {
	priv, pub, err := GenerateKeyPair()

	assert.NoError(t, err)

	assert.NotEmpty(t, priv)
	assert.NotEmpty(t, pub)

	assert.Regexp(t, regexp.MustCompile("^-----BEGIN RSA PRIVATE KEY-----.*"), priv)
	assert.Regexp(t, regexp.MustCompile("^-----BEGIN PUBLIC KEY-----.*"), pub)

}
