// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"testing"

	"gitea.dev/modules/test"
	"gitea.dev/modules/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configCheck is one expectation about the settings globals after a config load.
type configCheck interface {
	snapshot() (restore func()) // keeps cases independent of each other
	check(t *testing.T)
}

type configFieldCheck[T any] struct {
	key  string
	ptr  *T
	want T
}

func (c *configFieldCheck[T]) snapshot() func() { return test.MockVariableValue(c.ptr) }

func (c *configFieldCheck[T]) check(t *testing.T) {
	t.Helper()
	assert.Equal(t, c.want, *c.ptr, "config key %s", c.key)
}

// field expects the global at ptr to equal want after loading, key being the ini key it comes from.
// ptr must address a global directly, use fieldOf for values behind a pointer the loader replaces.
func field[T any](key string, ptr *T, want T) configCheck {
	return &configFieldCheck[T]{key: key, ptr: ptr, want: want}
}

type configValueCheck[T any] struct {
	key  string
	get  func() T
	want T
}

func (c *configValueCheck[T]) snapshot() func() { return func() {} }

func (c *configValueCheck[T]) check(t *testing.T) {
	t.Helper()
	assert.Equal(t, c.want, c.get(), "config key %s", c.key)
}

// fieldOf is field for values read through a pointer the loader replaces, it restores nothing
// so the test must guard the whole settings struct with test.MockVariableValue.
func fieldOf[T any](key string, get func() T, want T) configCheck {
	return &configValueCheck[T]{key: key, get: get, want: want}
}

type configGuard[T any] struct{ ptr *T }

func (c *configGuard[T]) snapshot() func() { return test.MockVariableValue(c.ptr) }

func (c *configGuard[T]) check(*testing.T) {}

// guard restores a global the loaders write but the case does not assert, so that loaders keeping
// the existing value for an absent key (MapTo, MustString(current)) still see the package default.
func guard[T any](ptr *T) configCheck {
	return &configGuard[T]{ptr: ptr}
}

type configTestCase struct {
	name    string // defaults to ini
	ini     string
	loaders []any                     // overrides the table loaders
	wantErr assert.ErrorAssertionFunc // nil means assert.NoError
	want    []configCheck
	check   func(t *testing.T) // for the rare assertion field cannot express
}

// testConfigLoad runs each case against loaders, which are load*From functions with or without an
// error return, applied in order to the same config. Use it when the test is "parse this ini, run
// these loaders, assert settings globals", not when it asserts a local or returned value, mutates
// files or env between loads, or tests the config provider itself.
//
// Cases restore the globals they assert, so they are order independent, but the loaders write more
// than any one case asserts: guard the whole settings struct with test.MockVariableValue as well.
func testConfigLoad(t *testing.T, loaders []any, cases []configTestCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(util.IfZero(c.name, c.ini), func(t *testing.T) {
			for _, w := range c.want {
				defer w.snapshot()()
			}

			cfg, err := NewConfigProviderFromData(c.ini)
			require.NoError(t, err)

			caseLoaders := loaders
			if c.loaders != nil {
				caseLoaders = c.loaders // util.IfZero needs comparable, []any is not
			}
			var loadErr error
			for _, loader := range caseLoaders {
				if loadErr = configLoader(t, loader)(cfg); loadErr != nil {
					break
				}
			}

			wantErr := c.wantErr
			if wantErr == nil {
				wantErr = assert.NoError
			}
			if !wantErr(t, loadErr) {
				return
			}

			for _, w := range c.want {
				w.check(t)
			}
			if c.check != nil {
				c.check(t)
			}
		})
	}
}

func configLoader(t *testing.T, loader any) func(ConfigProvider) error {
	t.Helper()
	switch f := loader.(type) {
	case func(ConfigProvider):
		return func(cfg ConfigProvider) error { f(cfg); return nil }
	case func(ConfigProvider) error:
		return f
	}
	t.Fatalf("unsupported config loader %T", loader)
	return nil
}
