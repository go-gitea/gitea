// +build !js

package ioutil

import "io"

func Pipe() (PipeReader, PipeWriter) {
	return io.Pipe()
}
