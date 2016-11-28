package themis

import "github.com/juju/errors"

func errorEqual(err1, err2 error) bool {
	if err1 == err2 {
		return true
	}

	e1 := errors.Cause(err1)
	e2 := errors.Cause(err2)

	if e1 == e2 {
		return true
	}

	if e1 == nil || e2 == nil {
		return e1 == e2
	}

	return e1.Error() == e2.Error()
}
