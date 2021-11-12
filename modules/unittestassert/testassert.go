// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittestassert

import (
	"code.gitea.io/gitea/modules/unittestapi"
	"github.com/stretchr/testify/assert"
)

// TestifyAsserter uses "stretchr/testify/assert" to do assert
type TestifyAsserter struct {
	t unittestapi.Tester
}

//Errorf Errorf
func (ta TestifyAsserter) Errorf(format string, args ...interface{}) {
	ta.t.Errorf(format, args)
}

//NoError NoError
func (ta TestifyAsserter) NoError(err error, msgAndArgs ...interface{}) bool {
	return assert.NoError(ta, err, msgAndArgs...)
}

//EqualValues EqualValues
func (ta TestifyAsserter) EqualValues(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return assert.EqualValues(ta, expected, actual, msgAndArgs...)
}

//Equal Equal
func (ta TestifyAsserter) Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return assert.Equal(ta, expected, actual, msgAndArgs...)
}

//True True
func (ta TestifyAsserter) True(value bool, msgAndArgs ...interface{}) bool {
	return assert.True(ta, value, msgAndArgs...)
}

//False False
func (ta TestifyAsserter) False(value bool, msgAndArgs ...interface{}) bool {
	return assert.False(ta, value, msgAndArgs...)
}

//NewTestifyAsserter returns a new asserter
func NewTestifyAsserter(t unittestapi.Tester) unittestapi.Asserter {
	return &TestifyAsserter{t: t}
}
