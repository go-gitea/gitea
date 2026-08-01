// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"strconv"
	"strings"
)

// FileNameJoinFields joins the fields by separator "-" as a safe filename,
// the last field is used as extname (no sep is added before it).
// This function does its best to avoid the filename exceeding the max filename length (255 bytes on Linux),
// Each field needs at least about 3 bytes, if the caller passes too many fields (e.g.: 100 fields),
// then it's unavoidable, just do not write such code in real world.
func FileNameJoinFields(fields ...any) string {
	// linux max filename length is 255, we leave some space for the separators and extname
	const maxFilenameLen = 210
	return fileNameJoinFields(maxFilenameLen, fields...)
}

func fileNameJoinFields(stemNameLimit int, fields ...any) string {
	// stemNameLimit is just a suggested limit, when adding more fields, the length might still exceed a little
	sb := strings.Builder{}
	for i, f := range fields {
		var field string
		switch v := f.(type) {
		case string:
			field = v
		case int64:
			field = strconv.FormatInt(v, 10)
		default:
			field = fmt.Sprint(v)
		}
		field = PathJoinRelX(field)
		field = PathNameValidator().InvalidChars.ReplaceAllString(field, "_")
		if i < len(fields)-1 {
			field = strings.ReplaceAll(strings.ReplaceAll(field, ".", "_"), "-", "_")
			estimatedRemainingLen := (len(fields) - 1 - i) * 3
			estimatedLimit := stemNameLimit - estimatedRemainingLen
			if sb.Len()+len(field) > estimatedLimit {
				field = TruncateStringBytes(field, estimatedLimit-sb.Len()-2) + "__"
			}
			sb.WriteString(field)
			if i < len(fields)-2 {
				sb.WriteString("-")
			}
		} else {
			// last field is extname, no need to do more processing
			sb.WriteString(field)
		}
	}
	return sb.String()
}
