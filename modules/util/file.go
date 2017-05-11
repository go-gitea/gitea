// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package file

// formatFileSizeToMB formats the filesize
func formatFileSizeToMB(bytes int64) string {
	result := float64(bytes) / float64(1048576)
	var format string
	if result < 0.01 {
		format = "%.2f"
		result = 0.01
	} else if result < 0.1 {
		format = "%.2f"
	} else {
		format = "%.1f"
	}
	return fmt.Sprintf(format, result) + " MB"
}

// getFileSize returns file size
func getFileSize(path string) int64 {
	stats, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return stats.Size()
}