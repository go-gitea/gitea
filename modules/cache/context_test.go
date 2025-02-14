// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cache

import (
	"context"
	"testing"
	"time"

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
	RemoveContextData(ctx, field, "my_config2") // remove a non-exist key

	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v)

	vInt, err := GetWithContextCache(ctx, field, "my_config1", func() (int, error) {
		return 1, nil
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, vInt)

	v = GetContextData(ctx, field, "my_config1")
	assert.EqualValues(t, 1, v)

	now := timeNow
	defer func() {
		timeNow = now
	}()
	timeNow = func() time.Time {
		return now().Add(5 * time.Minute)
	}
	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v)
}

func TestWithNoCacheContext(t *testing.T) {
	ctx := context.Background()

	const field = "system_setting"

	v := GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v)
	SetContextData(ctx, field, "my_config1", 1)
	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v) // still no cache

	ctx = WithCacheContext(ctx)
	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v)
	SetContextData(ctx, field, "my_config1", 1)
	v = GetContextData(ctx, field, "my_config1")
	assert.NotNil(t, v)

	ctx = WithNoCacheContext(ctx)
	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v)
	SetContextData(ctx, field, "my_config1", 1)
	v = GetContextData(ctx, field, "my_config1")
	assert.Nil(t, v) // still no cache
}
