// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package storage

import (
	"fmt"
	"io"
	"net/url"
	"os"
)

var uninitializedStorage = discardStorage("uninitialized storage")

type discardStorage string

func (s discardStorage) Open(_ string) (Object, error) {
	return nil, fmt.Errorf("%s", s)
}

func (s discardStorage) Save(_ string, _ io.Reader, _ int64) (int64, error) {
	return 0, fmt.Errorf("%s", s)
}

func (s discardStorage) Stat(_ string) (os.FileInfo, error) {
	return nil, fmt.Errorf("%s", s)
}

func (s discardStorage) Delete(_ string) error {
	return fmt.Errorf("%s", s)
}

func (s discardStorage) URL(_, _ string, _ url.Values) (*url.URL, error) {
	return nil, fmt.Errorf("%s", s)
}

func (s discardStorage) IterateObjects(_ string, _ func(string, Object) error) error {
	return fmt.Errorf("%s", s)
}
