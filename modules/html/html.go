// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package html

// ParseSizeAndClass get size and class from string with default values
// If present, "others" expects the new size first and then the classes to use
func ParseSizeAndClass(defaultSize int, defaultClass string, others ...any) (int, string) {
	size := defaultSize
	if len(others) >= 1 {
		if v, ok := others[0].(int); ok && v != 0 {
			size = v
		}
	}
	class := defaultClass
	if len(others) >= 2 {
		if v, ok := others[1].(string); ok && v != "" {
			if class != "" {
				class += " "
			}
			class += v
		}
	}
	return size, class
}
