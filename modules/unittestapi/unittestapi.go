package unittestapi

type Tester interface {
	Errorf(format string, args ...interface{})
}

type Asserter interface {
	Tester
	NoError(err error, msgAndArgs ...interface{}) bool
	EqualValues(expected, actual interface{}, msgAndArgs ...interface{}) bool
	Equal(expected, actual interface{}, msgAndArgs ...interface{}) bool
	True(value bool, msgAndArgs ...interface{}) bool
	False(value bool, msgAndArgs ...interface{}) bool
}

var newAsserterFunc func(t Tester) Asserter

func NewAsserter(t Tester) Asserter {
	if newAsserterFunc == nil {
		panic("the newAsserterFunc is not set. you can only use assert in unit test.")
	}
	return newAsserterFunc(t)
}

func SetNewAsserterFunc(f func(t Tester) Asserter) {
	newAsserterFunc = f
}
