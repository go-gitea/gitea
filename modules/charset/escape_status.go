// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package charset

// EscapeStatus represents the findings of the unicode escaper
type EscapeStatus struct {
	Escaped      bool
	HasError     bool
	HasBadRunes  bool
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
	st.HasError = st.HasError || other.HasError
	st.HasBadRunes = st.HasBadRunes || other.HasBadRunes
	st.HasAmbiguous = st.HasAmbiguous || other.HasAmbiguous
	st.HasInvisible = st.HasInvisible || other.HasInvisible
	return st
}
