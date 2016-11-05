// +build !bindata

package bindata

import (
	"fmt"
	"path/filepath"
	"io/ioutil"
	"strings"
)

func resolveName(name string) (string, error) {

	name = strings.Replace(name, "\\", "/", -1) // needed ?

	// TODO: cache this
	dir, err := filepath.Abs(".")
	if err != nil {
		return "", fmt.Errorf("%v", err)
	}
	for {
		_, err = ioutil.ReadDir(filepath.Join(dir, "conf"))
		if err == nil {
			// TODO: check if the file is a directory
			break
		}
		// TODO: check that the error is a "file not found" error ?
		newdir := filepath.Join(dir, "..")
		if dir == newdir {
			return "", fmt.Errorf("Could not find directory containing 'conf'")
		}
		dir = newdir
	}

	return filepath.Join(dir,name), nil
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {

	canonicalName, err := resolveName(name)
	if err != nil {
		return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
	}

	// TODO: cache this
	dat, err := ioutil.ReadFile(canonicalName)

	if err != nil {
		return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
	}
	return dat, nil
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	canonicalName, err := resolveName(name)
	if err != nil {
		return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
	}

	// TODO: cache these
	files, err := ioutil.ReadDir(canonicalName)
	if err != nil {
		return nil, fmt.Errorf("Error reading directory %s: ", err)
	}

	rv := make([]string, 0, len(files))
	for _, f := range files {
		rv = append(rv, f.Name())
	}

	return rv, nil
}
