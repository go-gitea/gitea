// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// CreatePushMirrorOption represents need information to create a push mirror of a repository.
type CreatePushMirrorOption struct {
	RemoteAddress  string `json:"remoteAddress"`
	RemoteUsername string `json:"remoteUsername"`
	RemotePassword string `json:"remotePassword"`
	Interval       string `json:"interval"`
}

// PushMirror represents information of a push mirror
// swagger:model
type PushMirror struct {
	RepoName       string `json:"repoName"`
	RemoteName     string `json:"remoteName"`
	RemoteAddress  string `json:"remoteAddress"`
	CreatedUnix    string `json:"created"`
	LastUpdateUnix string `json:"lastUpdate"`
	LastError      string `json:"lastError"`
	Interval       string `json:"interval"`
}
