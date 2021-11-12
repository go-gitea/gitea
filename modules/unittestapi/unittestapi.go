// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package unittestapi

//Tester is the same as TestingT in "stretchr/testify/assert"
//Tester can be used in non-unit-test code (ex: models.CheckConsistencyFor), it is used to isolate dependencies
type Tester interface {
	Errorf(format string, args ...interface{})
}

//Asserter can be used in non-unit-test code (ex: models.CheckConsistencyFor), it is used to isolate dependencies
type Asserter interface {
	Tester
	NoError(err error, msgAndArgs ...interface{}) bool
	EqualValues(expected, actual interface{}, msgAndArgs ...interface{}) bool
	Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool
	True(value bool, msgAndArgs ...interface{}) bool
	False(value bool, msgAndArgs ...interface{}) bool
}

var newAsserterFunc func(t Tester) Asserter

//NewAsserter returns a new asserter, only works in unit test
func NewAsserter(t Tester) Asserter {
	if newAsserterFunc == nil {
		panic("the newAsserterFunc is not set. you can only use assert in unit test.")
	}
	return newAsserterFunc(t)
}

//SetNewAsserterFunc in unit test, the asserter must be set first
func SetNewAsserterFunc(f func(t Tester) Asserter) {
	newAsserterFunc = f
}
