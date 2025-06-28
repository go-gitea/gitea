// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// SubModule is a reference on git repository
type SubModule struct {
	Path   string
	URL    string
	Branch string // this field is newly added but not really used
}

// configParseSubModules this is not a complete parse for gitmodules file, it only
// parses the url and path of submodules. At the moment it only parses well-formed gitmodules files.
// In the future, there should be a complete implementation of https://git-scm.com/docs/git-config#_syntax
func configParseSubModules(r io.Reader) (*ObjectCache[*SubModule], error) {
	var subModule *SubModule
	subModules := newObjectCache[*SubModule]()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header [section]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if subModule != nil {
				subModules.Set(subModule.Path, subModule)
			}
			if strings.HasPrefix(line, "[submodule") {
				subModule = &SubModule{}
			} else {
				subModule = nil
			}
			continue
		}

		if subModule == nil {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "path":
			subModule.Path = value
		case "url":
			subModule.URL = value
		case "branch":
			subModule.Branch = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}
	if subModule != nil {
		subModules.Set(subModule.Path, subModule)
	}
	return subModules, nil
}
