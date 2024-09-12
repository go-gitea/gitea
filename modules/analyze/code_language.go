// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package analyze

import (
	"path/filepath"

	"github.com/go-enry/go-enry/v2"
)

// GetCodeLanguage detects code language based on file name and content
func GetCodeLanguage(filename string, content []byte) string {
	if language, ok := enry.GetLanguageByExtension(filename); ok {
		return language
	}

	if language, ok := enry.GetLanguageByFilename(filename); ok {
		return language
	}

	if len(content) == 0 {
		return enry.OtherLanguage
	}

	return enry.GetLanguage(filepath.Base(filename), content)
}
