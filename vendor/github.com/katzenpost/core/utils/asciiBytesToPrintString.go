// asciiBytesToPrintString.go - Convert a binary buffer to a string.
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

import "unicode"

// ASCIIBytesToPrintString converts the buffer b to the closest ASCII string
// equivalent, substituting '*' for unprintable characters.
func ASCIIBytesToPrintString(b []byte) string {
	r := make([]byte, 0, len(b))

	// This should *never* be used in production, since it attempts to give a
	// printable representation of a byte sequence for debug logging, and it's
	// slow.
	for _, v := range b {
		if v <= unicode.MaxASCII && unicode.IsPrint(rune(v)) {
			r = append(r, v)
		} else {
			r = append(r, '*') // At least I didn't pick `:poop:`.
		}
	}
	return string(r)
}
