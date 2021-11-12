package unittestassert

import (
	"code.gitea.io/gitea/modules/unittestapi"
	"github.com/stretchr/testify/assert"
)

type TestifyAsserter struct {
	t unittestapi.Tester
}

func (ta TestifyAsserter) Errorf(format string, args ...interface{}) {
	ta.t.Errorf(format, args)
}

func (ta TestifyAsserter) NoError(err error, msgAndArgs ...interface{}) bool {
	return assert.NoError(ta, err, msgAndArgs...)
}

func (ta TestifyAsserter) EqualValues(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return assert.EqualValues(ta, expected, actual, msgAndArgs...)
}

func (ta TestifyAsserter) Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool {
	return assert.Equal(ta, expected, actual, msgAndArgs...)
}

func (ta TestifyAsserter) True(value bool, msgAndArgs ...interface{}) bool {
	return assert.True(ta, value, msgAndArgs...)
}

func (ta TestifyAsserter) False(value bool, msgAndArgs ...interface{}) bool {
	return assert.False(ta, value, msgAndArgs...)
}

func NewTestifyAsserter(t unittestapi.Tester) unittestapi.Asserter {
	return &TestifyAsserter{t: t}
}
