// mkDataDir.go - Create a data directory.
// Copyright (C) 2017  Yawning Angel.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package utils

import (
	"fmt"
	"os"
)

// MkDataDir creates a data directory of appropriate (paranoid) permissions if
// it does not exist, and validates that existing directories have the intended
// permissions if it does exist.
func MkDataDir(f string) error {
	const dirMode = os.ModeDir | 0700

	if fi, err := os.Lstat(f); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to stat() dir: %v", err)
		}
		if err = os.Mkdir(f, dirMode); err != nil {
			return fmt.Errorf("failed to create dir: %v", err)
		}
	} else {
		if !fi.IsDir() {
			return fmt.Errorf("dir '%v' is not a directory", f)
		}
		if fi.Mode() != dirMode {
			return fmt.Errorf("dir '%v' has invalid permissions '%v", f, fi.Mode())
		}
	}
	return nil
}
