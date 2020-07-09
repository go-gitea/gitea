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
	Permissions               *Permission      `json:"permissions,omitempty"`
	InternalTracker           *InternalTracker `json:"internal_tracker,omitempty"`
	ExternalTracker           *ExternalTracker `json:"external_tracker,omitempty"`
	ExternalWiki              *ExternalWiki    `json:"external_wiki,omitempty"`
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
				Description: "Is this repository a template",
			},
			"mirror": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Is this repository a mirror",
			},
			"archived": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Is this repository archived",
			},
			"has_issues": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Does this repository contain issues",
			},
			"has_wiki": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Does this repository have a wiki",
			},
			"has_pull_requests": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Does this repository have pull requests",
			},
			"ignore_whitespace_conflicts": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Does this repository ignore whitespace for conflicts",
			},
			"allow_merge_commits": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Is commit merging enabled",
			},
			"allow_rebase": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Is rebasing enabled",
			},
			"allow_rebase_explicit": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Is rebasing with explicit merge commits (--no-ff) enabled",
			},
			"allow_squash_merge": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Is squashing to merge commits enabled",
			},
			"internal": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Is visibility of repository set to private",
			},
			"size": &graphql.Field{
				Type: graphql.Int,
				Description: "Repository size",
			},
			"stars_count": &graphql.Field{
				Type: graphql.Int,
				Description: "Number of stars",
			},
			"forks_count": &graphql.Field{
				Type: graphql.Int,
				Description: "Number of forks",
			},
			"watchers_count": &graphql.Field{
				Type: graphql.Int,
				Description: "Number of watchers",
			},
			"open_issues_count": &graphql.Field{
				Type: graphql.Int,
				Description: "Number of open issues",
			},
			"open_pr_counter": &graphql.Field{
				Type: graphql.Int,
				Description: "Number of open pull requests",
			},
			"release_counter": &graphql.Field{
				Type: graphql.Int,
				Description: "Number of releases",
			},
			"html_url": &graphql.Field{
				Type: graphql.String,
				Description: "HTML url of repository",
			},
			"ssh_url": &graphql.Field{
				Type: graphql.String,
				Description: "SSH url of repository",
			},
			"clone_url": &graphql.Field{
				Type: graphql.String,
				Description: "Clone url of repository",
			},
			"website": &graphql.Field{
				Type: graphql.String,
				Description: "The repository's website address",
			},
			"default_branch": &graphql.Field{
				Type: graphql.String,
				Description: "The default branch",
			},
			"avatar_url": &graphql.Field{
				Type: graphql.String,
				Description: "Avatar url for repository",
			},
			"created_at": &graphql.Field{
				Type:        graphql.DateTime,
				Description: "Datetime repository created",
			},
			"updated_at": &graphql.Field{
				Type:        graphql.DateTime,
				Description: "Dateime repository last updated",
			},

/*

	"parent": &graphql.Field{
		Type:        repository,
		Description: "Parent repository",
	},


 */

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

func init() {
	//direct circular references not allowed, so adding here as a workaround
	//reference: https://github.com/graphql-go/graphql/issues/164
	generalInfo.AddFieldConfig("parent", &graphql.Field{
		Type:        repository,
		Description: "Parent repository",
	})
}
