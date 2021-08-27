package archiver

import (
	"fmt"
	"strings"
)

// IllegalPathError is an error returned when an illegal
// path is detected during the archival process.
//
// By default, only the Filename is showed on error, but you might
// also get the absolute value of the invalid path on the AbsolutePath
// field.
type IllegalPathError struct {
	AbsolutePath string
	Filename     string
}

func (err *IllegalPathError) Error() string {
	return fmt.Sprintf("illegal file path: %s", err.Filename)
}

// IsIllegalPathError returns true if the provided error is of
// the type IllegalPathError.
func IsIllegalPathError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "illegal file path: ")
}
