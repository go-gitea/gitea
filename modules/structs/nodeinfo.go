// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// NodeInfo contains standardized way of exposing metadata about a server running one of the distributed social networks
type NodeInfo struct {
	Version           string           `json:"version"`
	Software          NodeInfoSoftware `json:"software"`
	Protocols         []string         `json:"protocols"`
	Services          NodeInfoServices `json:"services"`
	OpenRegistrations bool             `json:"openRegistrations"`
	Usage             NodeInfoUsage    `json:"usage"`
	Metadata          struct{}         `json:"metadata"`
}

// NodeInfoSoftware contains Metadata about server software in use
type NodeInfoSoftware struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Homepage   string `json:"homepage"`
}

// NodeInfoServices contains the third party sites this server can connect to via their application API
type NodeInfoServices struct {
	Inbound  []string `json:"inbound"`
	Outbound []string `json:"outbound"`
}

// NodeInfoUsage contains usage statistics for this server
type NodeInfoUsage struct {
	Users         NodeInfoUsageUsers `json:"users"`
	LocalPosts    int                `json:"localPosts,omitempty"`
	LocalComments int                `json:"localComments,omitempty"`
}

// NodeInfoUsageUsers contains statistics about the users of this server
type NodeInfoUsageUsers struct {
	Total          int `json:"total,omitempty"`
	ActiveHalfyear int `json:"activeHalfyear,omitempty"`
	ActiveMonth    int `json:"activeMonth,omitempty"`
}
