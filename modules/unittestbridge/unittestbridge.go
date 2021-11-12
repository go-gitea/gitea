// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittestbridge

// Usage: generally, non-unit-test code shouldn't depend on unit test code.
// However, we have some code like models.CheckConsistencyFor, which need to do some unit test works.
// Now we can not decouple models.CheckConsistencyFor from unit test code easily (cycle-import reasons).
// So we introduce this `unit test bridge`:
// * When a release binary is built, no testing/assert framework would be compiled into the binary, and CheckConsistencyFor won't run unit test related code
// * When a unit test binary is built, the unit test code will init this bridge, then CheckConsistencyFor can run unit test related code
//
// Tester/Assert are intermediate interfaces, they should NOT be used in new code.
// One day, if CheckConsistencyFor is clean enough, we can remove these intermediate interfaces.

// Tester is the same as TestingT in "stretchr/testify/assert"
// Tester can be used in non-unit-test code (ex: models.CheckConsistencyFor), it is used to isolate dependencies
type Tester interface {
	Errorf(format string, args ...interface{})
}

// Asserter can be used in non-unit-test code (ex: models.CheckConsistencyFor), it is used to isolate dependencies
type Asserter interface {
	Tester
	NoError(err error, msgAndArgs ...interface{}) bool
	EqualValues(expected, actual interface{}, msgAndArgs ...interface{}) bool
	Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool
	True(value bool, msgAndArgs ...interface{}) bool
	False(value bool, msgAndArgs ...interface{}) bool
}

var newAsserterFunc func(t Tester) Asserter

// NewAsserter returns a new asserter, only works in unit test
func NewAsserter(t Tester) Asserter {
	if newAsserterFunc == nil {
		panic("the newAsserterFunc is not set. you can only use assert in unit test.")
	}
	return newAsserterFunc(t)
}

// SetNewAsserterFunc in unit test, the asserter must be set first
func SetNewAsserterFunc(f func(t Tester) Asserter) {
	newAsserterFunc = f
}
