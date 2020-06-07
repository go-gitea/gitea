package brotli

/* Copyright 2010 Google Inc. All Rights Reserved.

   Distributed under MIT license.
   See file LICENSE for detail or copy at https://opensource.org/licenses/MIT
*/

/* Function to find maximal matching prefixes of strings. */
func findMatchLengthWithLimit(s1 []byte, s2 []byte, limit uint) uint {
	var matched uint = 0
	for matched < limit && s1[matched] == s2[matched] {
		matched++
	}
	return matched
}
