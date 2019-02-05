// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package options

//go:generate packr2
//go:generate sh -c "echo '// +build bindata' | cat - packrd/packed-packr.go > packrd/packed-packr.go.bak"
//go:generate sh -c "echo '// +build bindata' | cat - options-packr.go > options-packr.go.bak"
//go:generate sh -c "mv packrd/packed-packr.go.bak packrd/packed-packr.go"
//go:generate sh -c "mv options-packr.go.bak options-packr.go"

type directorySet map[string][]string

func (s directorySet) Add(key string, value []string) {
	_, ok := s[key]

	if !ok {
		s[key] = make([]string, 0, len(value))
	}

	s[key] = append(s[key], value...)
}

func (s directorySet) Get(key string) []string {
	_, ok := s[key]

	if ok {
		result := []string{}
		seen := map[string]string{}

		for _, val := range s[key] {
			if _, ok := seen[val]; !ok {
				result = append(result, val)
				seen[val] = val
			}
		}

		return result
	}

	return []string{}
}

func (s directorySet) AddAndGet(key string, value []string) []string {
	s.Add(key, value)
	return s.Get(key)
}

func (s directorySet) Filled(key string) bool {
	return len(s[key]) > 0
}
