// This work is subject to the CC0 1.0 Universal (CC0 1.0) Public Domain Dedication
// license. Its contents can be found at:
// http://creativecommons.org/publicdomain/zero/1.0/

package bindata

import (
	"fmt"
	"io"
)

// writeDebug writes the debug code file.
func writeDebug(w io.Writer, toc []Asset) error {
	err := writeDebugHeader(w)
	if err != nil {
		return err
	}

	for i := range toc {
		err = writeDebugAsset(w, &toc[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// writeDebugHeader writes output file headers.
// This targets debug builds.
func writeDebugHeader(w io.Writer) error {
	_, err := fmt.Fprintf(w, `import (
	"fmt"
	"io/ioutil"
	"strings"
)

// bindata_read reads the given file from disk. It returns an error on failure.
func bindata_read(path, name string) ([]byte, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("Error reading asset %%s at %%s: %%v", name, path, err)
	}
	return buf, err
}

`)
	return err
}

// writeDebugAsset write a debug entry for the given asset.
// A debug entry is simply a function which reads the asset from
// the original file (e.g.: from disk).
func writeDebugAsset(w io.Writer, asset *Asset) error {
	_, err := fmt.Fprintf(w, `// %s reads file data from disk. It returns an error on failure.
func %s() ([]byte, error) {
	return bindata_read(
		%q,
		%q,
	)
}

`, asset.Func, asset.Func, asset.Path, asset.Name)
	return err
}
