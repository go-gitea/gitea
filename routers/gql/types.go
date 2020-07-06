// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import "github.com/graphql-go/graphql"

var repository = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "repository",
		Fields: graphql.Fields{
			"general_info": &graphql.Field{
				Type:        generalInfo,
				Description: "General information about a repository",
			},
			"branches": &graphql.Field{
				Type:        graphql.NewList(branch),
				Description: "Branches contained within a repostory",
				Resolve: BranchesResolver,
			},
			"collaborators": &graphql.Field{
				Type:        graphql.NewList(user),
				Description: "The repository's collaborators",
				Resolve: CollaboratorsResolver,
			},
		},
	},
)

/*
//TODO add all these
// Repository represents a repository
type Repository struct {
	Parent        *Repository `json:"parent"`
	Mirror        bool        `json:"mirror"`
	Size          int         `json:"size"`
	HTMLURL       string      `json:"html_url"`
	SSHURL        string      `json:"ssh_url"`
	CloneURL      string      `json:"clone_url"`
	OriginalURL   string      `json:"original_url"`
	Website       string      `json:"website"`
	Stars         int         `json:"stars_count"`
	Forks         int         `json:"forks_count"`
	Watchers      int         `json:"watchers_count"`
	OpenIssues    int         `json:"open_issues_count"`
	OpenPulls     int         `json:"open_pr_counter"`
	Releases      int         `json:"release_counter"`
	DefaultBranch string      `json:"default_branch"`
	Archived      bool        `json:"archived"`
	// swagger:strfmt date-time
	Created time.Time `json:"created_at"`
	// swagger:strfmt date-time
	Updated                   time.Time        `json:"updated_at"`
	Permissions               *Permission      `json:"permissions,omitempty"`
	HasIssues                 bool             `json:"has_issues"`
	InternalTracker           *InternalTracker `json:"internal_tracker,omitempty"`
	ExternalTracker           *ExternalTracker `json:"external_tracker,omitempty"`
	HasWiki                   bool             `json:"has_wiki"`
	ExternalWiki              *ExternalWiki    `json:"external_wiki,omitempty"`
	HasPullRequests           bool             `json:"has_pull_requests"`
	IgnoreWhitespaceConflicts bool             `json:"ignore_whitespace_conflicts"`
	AllowMerge                bool             `json:"allow_merge_commits"`
	AllowRebase               bool             `json:"allow_rebase"`
	AllowRebaseMerge          bool             `json:"allow_rebase_explicit"`
	AllowSquash               bool             `json:"allow_squash_merge"`
	AvatarURL                 string           `json:"avatar_url"`
}




*/

// generalInfo describes general information about a repository
var generalInfo = graphql.NewObject(
	graphql.ObjectConfig{
		Name:        "general_info",
		Description: "General Information about a repository",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        graphql.Int,
				Description: "The id of the repository",
			},
			"owner": &graphql.Field{
				Type:        user,
				Description: "Owner of the repository",
			},
			"name": &graphql.Field{
				Type: graphql.String,
				Description: "Name of the repository",
			},
			"full_name": &graphql.Field{
				Type: graphql.String,
				Description: "Full name of the repository",
			},
			"description": &graphql.Field{
				Type: graphql.String,
				Description: "Description of the repository",
			},
			"empty": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Whether the repository is empty or not.",
			},
			"private": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Whether the repository is private or not",
			},
			"fork": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Whether the repository is a fork or not",
			},
			"template": &graphql.Field{
				Type: graphql.Boolean,
			},
		},
	},
)

/*
type Branch struct {
	Commit                        *PayloadCommit `json:"commit"`
	Protected                     bool           `json:"protected"`
	RequiredApprovals             int64          `json:"required_approvals"`
	EnableStatusCheck             bool           `json:"enable_status_check"`
	StatusCheckContexts           []string       `json:"status_check_contexts"`
	UserCanPush                   bool           `json:"user_can_push"`
	UserCanMerge                  bool           `json:"user_can_merge"`
	EffectiveBranchProtectionName string         `json:"effective_branch_protection_name"`
}
*/

//branch describes a branch
var branch = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "branch",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type: graphql.String,
				Description: "name of the branch",
			},
		},
	},
)

//user describes a user
var user = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "user",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        graphql.Int,
				Description: "the user's id",
			},
			"username": &graphql.Field{
				Type:        graphql.String,
				Description: "the user's username",
			},
			"full_name": &graphql.Field{
				Type:        graphql.String,
				Description: "the user's full name",
			},
			"email": &graphql.Field{
				Type:        graphql.String,
				Description: "the user's email",
			},
			"avatar_url": &graphql.Field{
				Type:        graphql.String,
				Description: "URL to the user's avatar",
			},
			"language": &graphql.Field{
				Type:        graphql.String,
				Description: "User locale",
			},
			"is_admin": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "Is the user an administrator",
			},
			"last_login": &graphql.Field{
				Type:        graphql.DateTime,
				Description: "the user's last login",
			},
			"created": &graphql.Field{
				Type:        graphql.DateTime,
				Description: "datetime user created",
			},
		},
	},
)
