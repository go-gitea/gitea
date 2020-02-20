// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package analyze

import (
	"path/filepath"

	"github.com/src-d/enry/v2"
)

// GetCodeLanguageWithCallback detects code language based on file name and content using callback
func GetCodeLanguageWithCallback(filename string, contentFunc func() ([]byte, error)) string {
	if language, ok := enry.GetLanguageByExtension(filename); ok {
		return language
	}

	if language, ok := enry.GetLanguageByFilename(filename); ok {
		return language
	}

	content, err := contentFunc()
	if err != nil {
		return enry.OtherLanguage
	}

	return enry.GetLanguage(filepath.Base(filename), content)
}

// GetCodeLanguage detects code language based on file name and content
func GetCodeLanguage(filename string, content []byte) string {
	return GetCodeLanguageWithCallback(filename, func() ([]byte, error) {
		return content, nil
	})
}
