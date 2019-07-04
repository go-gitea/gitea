// This work is subject to the CC0 1.0 Universal (CC0 1.0) Public Domain Dedication
// license. Its contents can be found at:
// http://creativecommons.org/publicdomain/zero/1.0/

package bindata

// Asset holds information about a single asset to be processed.
type Asset struct {
	Path string // Full file path.
	Name string // Key used in TOC -- name by which asset is referenced.
	Func string // Function name for the procedure returning the asset contents.
}

// Implement sort.Interface for []Asset based on Path field
type ByPath []Asset

func (v ByPath) Len() int           { return len(v) }
func (v ByPath) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v ByPath) Less(i, j int) bool { return v[i].Path < v[j].Path }
