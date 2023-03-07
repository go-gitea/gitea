// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithCacheContext(t *testing.T) {
	ctx := WithCacheContext(context.Background())

	v := GetContextData(ctx, "empty_field", "my_config1")
	assert.Nil(t, v)

	const field = "system_setting"
	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v)
	SetContextData(ctx, field, "my_config1", 1)
	v = GetContextData(ctx, field, "my_config1")
	assert.NotNil(t, v)
	assert.EqualValues(t, 1, v.(int))

	RemoveContextData(ctx, field, "my_config1")
	RemoveContextData(ctx, field, "my_config2") // remove an non-exist key

	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v)

	vInt, err := GetWithContextCache(ctx, field, "my_config1", func() (int, error) {
		return 1, nil
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, vInt)

	v = GetContextData(ctx, field, "my_config1")
	assert.EqualValues(t, 1, v)
}
