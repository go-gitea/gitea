// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"os"
)

// FormatFileSize formats the filesize
func FormatFileSize(bytes int64) string {
	var result float64
	var format string
	var measure string
	if bytes < 10240 { // bytes
		result = float64(bytes) / float64(1048)
		measure = "Bytes"
	} else if bytes < 1048576 { // kbytes
		result = float64(bytes) / float64(102400)
		measure = "KB"
	} else if bytes < 1048576000 { // mbytes
		result = float64(bytes) / float64(1048576)
		measure = "MB"
	} else if bytes < 1048576000000 { // gbytes
		result = float64(bytes) / float64(1048576000)
		measure = "GB"
	} else if bytes < 1048576000000000 { // tbytes
		result = float64(bytes) / float64(1048576000000)
		measure = "TB"
	}

	if result < 0.01 {
		format = "%.2f"
		result = 0.01
	} else if result < 0.1 {
		format = "%.2f"
	} else {
		format = "%.1f"
	}
	return fmt.Sprintf(format, result) + " " + measure
}

// GetFileSize returns file size
func GetFileSize(path string) int64 {
	stats, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return stats.Size()
}
