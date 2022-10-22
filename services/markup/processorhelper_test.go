// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package markup

import (
	"context"
	"testing"

	"code.gitea.io/gitea/models/unittest"

	"github.com/stretchr/testify/assert"
)

func TestProcessorHelper(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
	assert.True(t, ProcessorHelper().IsUsernameMentionable(context.Background(), "user10"))
	assert.False(t, ProcessorHelper().IsUsernameMentionable(context.Background(), "no-such-user"))
}
