// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

// NodeInfo contains standardized way of exposing metadata about a server running one of the distributed social networks
type NodeInfo struct {
	// Version specifies the schema version
	Version string `json:"version"`
	// Software contains information about the server software
	Software NodeInfoSoftware `json:"software"`
	// Protocols lists the protocols supported by this server
	Protocols []string `json:"protocols"`
	// Services contains third party services this server can connect to
	Services NodeInfoServices `json:"services"`
	// OpenRegistrations indicates if new user registrations are accepted
	OpenRegistrations bool `json:"openRegistrations"`
	// Usage contains server usage statistics
	Usage NodeInfoUsage `json:"usage"`
	// Metadata contains free form key value pairs for software specific values
	Metadata struct{} `json:"metadata"`
}

// NodeInfoSoftware contains Metadata about server software in use
type NodeInfoSoftware struct {
	// Name is the canonical name of this server software
	Name string `json:"name"`
	// Version is the version of this server software
	Version string `json:"version"`
	// Repository is the URL to the source code repository
	Repository string `json:"repository"`
	// Homepage is the URL to the homepage of this server software
	Homepage string `json:"homepage"`
}

// NodeInfoServices contains the third party sites this server can connect to via their application API
type NodeInfoServices struct {
	// Inbound lists services that can deliver content to this server
	Inbound []string `json:"inbound"`
	// Outbound lists services this server can deliver content to
	Outbound []string `json:"outbound"`
}

// NodeInfoUsage contains usage statistics for this server
type NodeInfoUsage struct {
	// Users contains user statistics
	Users NodeInfoUsageUsers `json:"users"`
	// LocalPosts is the total amount of posts made by users local to this server
	LocalPosts int `json:"localPosts,omitempty"`
	// LocalComments is the total amount of comments made by users local to this server
	LocalComments int `json:"localComments,omitempty"`
}

// NodeInfoUsageUsers contains statistics about the users of this server
type NodeInfoUsageUsers struct {
	// Total is the total amount of users on this server
	Total int `json:"total,omitempty"`
	// ActiveHalfyear is the amount of users that signed in at least once in the last 180 days
	ActiveHalfyear int `json:"activeHalfyear,omitempty"`
	// ActiveMonth is the amount of users that signed in at least once in the last 30 days
	ActiveMonth int `json:"activeMonth,omitempty"`
}
