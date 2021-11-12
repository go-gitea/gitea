// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittest

import (
	"code.gitea.io/gitea/modules/unittestbridge"
	"github.com/stretchr/testify/assert"
)

// For legacy code only, please refer to the `unittestbridge` package.

// TestifyAsserter uses "stretchr/testify/assert" to do assert
type TestifyAsserter struct {
	t unittestbridge.Tester
}

// Errorf assert Errorf
func (ta TestifyAsserter) Errorf(format string, args ...interface{}) {
	ta.t.Errorf(format, args)
}

// NoError assert NoError
func (ta TestifyAsserter) NoError(err error, msgAndArgs ...interface{}) bool {
	return assert.NoError(ta, err, msgAndArgs...)
}

// EqualValues assert EqualValues
func (ta TestifyAsserter) EqualValues(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return assert.EqualValues(ta, expected, actual, msgAndArgs...)
}

// Equal assert Equal
func (ta TestifyAsserter) Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return assert.Equal(ta, expected, actual, msgAndArgs...)
}

// True assert True
func (ta TestifyAsserter) True(value bool, msgAndArgs ...interface{}) bool {
	return assert.True(ta, value, msgAndArgs...)
}

// False assert False
func (ta TestifyAsserter) False(value bool, msgAndArgs ...interface{}) bool {
	return assert.False(ta, value, msgAndArgs...)
}

// InitUnitTestBridge init the unit test bridge. eg: models.CheckConsistencyFor can use testing and assert frameworks
func InitUnitTestBridge() {
	unittestbridge.SetNewAsserterFunc(func(t unittestbridge.Tester) unittestbridge.Asserter {
		return &TestifyAsserter{t: t}
	})
}
