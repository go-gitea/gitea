// +build js

package ioutil

import "github.com/acomagu/bufpipe"

func Pipe() (PipeReader, PipeWriter) {
	return bufpipe.New(nil)
}
