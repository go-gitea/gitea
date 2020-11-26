// +build bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"io/ioutil"
	"net/http"
)

// Static implements the macaron static handler for serving assets.
func Static(opts *Options) func(next http.Handler) http.Handler {
	opts.FileSystem = Assets
	// we don't need to pass the directory, because the directory var is only
	// used when in the options there is no FileSystem.
	return opts.staticHandler("")
}

// Asset returns a asset by path
func Asset(name string) ([]byte, error) {
	f, err := Assets.Open("/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}

// AssetNames returns an array of asset paths
func AssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	var results = make([]string, 0, len(realFS))
	for k := range realFS {
		results = append(results, k[1:])
	}
	return results
}

// AssetIsDir checks if an asset is a directory
func AssetIsDir(name string) (bool, error) {
	if f, err := Assets.Open("/" + name); err != nil {
		return false, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}
