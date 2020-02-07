// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package setting

import (
	"strings"
)

// https://s-randomfiles.s3.amazonaws.com/mime/allMimeTypes.txt

var commonMimeTypes = []struct {
	names []string
	types []string
}{
	{[]string{
		"zipped",
		"zip",
	}, []string{
		"application/zip",
		"application/x-zip-compressed",
		"multipart/x-zip",
		"application/gzip",
		"application/x-gzip",
		"application/x-gzip-compressed",
		"multipart/gzip",
		"application/x-compressed-tar",
		"application/x-gtar",
		"application/x-tgz",
		"application/rar",
		"application/x-rar-compressed",
		"application/x-7z-compressed",
		"application/x-compress",
	}},
	{[]string{
		"image",
		"picture",
	}, []string{
		"image/jpeg",
		"image/png",
		"image/apng",
		"image/bmp",
	}},
	{[]string{
		"txt",
		"text",
	}, []string{
		"text/plain",
	}},
	{[]string{
		"pdf",
	}, []string{
		"application/pdf",
	}},
}

func expandCommonMimeTypes(typeList string) string {
	individual := strings.Split(typeList, ",")
	list := make([]string, 0, len(individual))
individuals:
	for _, name := range individual {
		if !strings.Contains(name, "/") {
			normalized := strings.TrimSpace(strings.ToLower(name))
			for _, candidate := range commonMimeTypes {
				if isStringInSlice(normalized, candidate.names) {
					list = append(list, candidate.types...)
					continue individuals
				}
			}
		}
	}
	return strings.Join(list, ",")
}

// Can't import 'util' because it's a circular reference
func isStringInSlice(target string, slice []string) bool {
	for i := 0; i < len(slice); i++ {
		if slice[i] == target {
			return true
		}
	}
	return false
}
