// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"fmt"

	"gopkg.in/ini.v1"
)

// Addable defines something that can have string pairs added to it
type Addable interface {
	Add(key, value string)
}

// AddableFn is a type wrapper around a function to wrap it as an addable
type AddableFn func(key, value string)

func (fn AddableFn) Add(key, value string) {
	fn(key, value)
}

// Assert AddableFn makes an Addable
var _ Addable = AddableFn(func(_, _ string) {})

// AddFromIni reads the provided source as an ini file, collapsing sections in to keys
// and then adding them and their values one-by-one to the provided Addable
//
// if source is a string, then the file is loaded
// if source is a []byte, then the content is used
func AddFromIni(addable Addable, source interface{}) error {
	iniFile, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment:         true,
		UnescapeValueCommentSymbols: true,
	}, source)
	if err != nil {
		return fmt.Errorf("unable to load ini: %w", err)
	}
	iniFile.BlockMode = false

	for _, section := range iniFile.Sections() {
		for _, key := range section.Keys() {
			var trKey string
			if section.Name() == "" || section.Name() == "DEFAULT" {
				trKey = key.Name()
			} else {
				trKey = section.Name() + "." + key.Name()
			}
			addable.Add(trKey, key.Value())
		}
	}
	iniFile = nil
	return nil
}
