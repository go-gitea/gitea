// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"math"
	"os"
)

func humanateBytes(s uint64, base float64, sizes []string) string {
	if s < 10 {
		return fmt.Sprintf("%d B", s)
	}
	e := math.Floor(math.Log(float64(s)) / math.Log(base))
	suffix := sizes[int(e)]
	val := math.Floor(float64(s)/math.Pow(base, e)*10+0.5) / 10
	f := "%.1f %s"
	if val < 0.1 {
		f = "%.2f %s"
	}

	return fmt.Sprintf(f, val, suffix)
}

// FormatFileSize formats the filesize
func FormatFileSize(bytes int64) string {
	return humanateBytes(bytes, 1024, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"})
}

// GetFileSize returns file size
func GetFileSize(path string) int64 {
	stats, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return stats.Size()
}
