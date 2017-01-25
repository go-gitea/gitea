// Copyright 2014 Dustin Webber
// Copyright 2015 The Macaron Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package bindata is a helper module that allows to use in-memory static and template files for Macaron.
package bindata

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/elazarl/go-bindata-assetfs"
	"gopkg.in/macaron.v1"
)

const _VERSION = "0.1.1"

func Version() string {
	return _VERSION
}

type (
	templateFileSystem struct {
		files []macaron.TemplateFile
	}

	templateFile struct {
		name string
		data []byte
		ext  string
	}

	Options struct {
		// Asset should return content of file in path if exists
		Asset func(path string) ([]byte, error)
		// AssetDir should return list of files in the path
		AssetDir func(path string) ([]string, error)
		// AssetInfo should return the info of file in path if exists
		AssetInfo func(path string) (os.FileInfo, error)
		// AssetNames should return list of all asset names
		AssetNames func() []string
		// Prefix would be prepended to http requests
		Prefix string
	}
)

func Static(opt Options) *assetfs.AssetFS {
	fs := &assetfs.AssetFS{
		Asset:     opt.Asset,
		AssetDir:  opt.AssetDir,
		AssetInfo: opt.AssetInfo,
		Prefix:    opt.Prefix,
	}

	return fs
}

func (templates templateFileSystem) ListFiles() []macaron.TemplateFile {
	return templates.files
}

func (templates templateFileSystem) Get(name string) (io.Reader, error) {
	for i := range templates.files {
		if templates.files[i].Name()+templates.files[i].Ext() == name {
			return bytes.NewReader(templates.files[i].Data()), nil
		}
	}
	return nil, fmt.Errorf("file '%s' not found", name)
}

func (f *templateFile) Name() string {
	return f.name
}

func (f *templateFile) Data() []byte {
	return f.data
}

func (f *templateFile) Ext() string {
	return f.ext
}

func Templates(opt Options) templateFileSystem {
	fs := templateFileSystem{}
	fs.files = make([]macaron.TemplateFile, 0, 10)

	list := opt.AssetNames()

	for _, key := range list {
		ext := macaron.GetExt(key)

		data, err := opt.Asset(key)

		if err != nil {
			continue
		}

		name := (key[0 : len(key)-len(ext)])

		fs.files = append(fs.files, &templateFile{name, data, ext})
	}

	return fs
}
