// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package npm

// TagProperty is the name of the property for tag management
const TagProperty = "npm.tag"

// Metadata represents the metadata of a npm package
type Metadata struct {
	Scope                   string            `json:"scope"`
	Name                    string            `json:"name"`
	Description             string            `json:"description"`
	Author                  string            `json:"author"`
	License                 string            `json:"license"`
	ProjectURL              string            `json:"project_url"`
	Keywords                []string          `json:"keywords"`
	Dependencies            map[string]string `json:"dependencies"`
	DevelopmentDependencies map[string]string `json:"development_dependencies"`
	PeerDependencies        map[string]string `json:"peer_dependencies"`
	OptionalDependencies    map[string]string `json:"optional_dependencies"`
	Readme                  string            `json:"readme"`
}
