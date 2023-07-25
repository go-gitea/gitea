// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package html

// ParseSizeAndClass get size and class from string with default values
// If present, "others" expects the new size first and then the classes to use
func ParseSizeAndClass(defaultSize int, defaultClass string, others ...any) (int, string) {
	if len(others) == 0 {
		return defaultSize, defaultClass
	}

	size := defaultSize
	_size, ok := others[0].(int)
	if ok && _size != 0 {
		size = _size
	}

	if len(others) == 1 {
		return size, defaultClass
	}

	class := defaultClass
	if _class, ok := others[1].(string); ok && _class != "" {
		if defaultClass == "" {
			class = _class
		} else {
			class = defaultClass + " " + _class
		}
	}

	return size, class
}
