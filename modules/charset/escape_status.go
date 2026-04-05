// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package charset

// EscapeStatus represents the findings of the Unicode escaper
type EscapeStatus struct {
	Escaped      bool // it means that some characters were escaped, and they can also be unescaped back
	HasInvisible bool
	HasAmbiguous bool
}

// Or combines two EscapeStatus structs into one representing the conjunction of the two
func (status *EscapeStatus) Or(other *EscapeStatus) *EscapeStatus {
	st := status
	if status == nil {
		st = &EscapeStatus{}
	}
	st.Escaped = st.Escaped || other.Escaped
	st.HasAmbiguous = st.HasAmbiguous || other.HasAmbiguous
	st.HasInvisible = st.HasInvisible || other.HasInvisible
	return st
}
