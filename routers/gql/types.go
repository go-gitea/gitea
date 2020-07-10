// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"strconv"
)

//TODO need to create a custom scalar type for int64:
// can adapt from https://github.com/graphql-go/graphql/blob/master/examples/custom-scalar-type/main.go
// for now just using INT from graphql, but need to go back and make sure I'm right...


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

// generalInfo describes general information about a repository
var generalInfo = graphql.NewObject(
	graphql.ObjectConfig{
		Name:        "general_info",
		Description: "General Information about a repository",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        gqlInt64,
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
				Description: "Datetime repository last updated",
			},
			"permissions": &graphql.Field{
				Type:        permission,
				Description: "Repository permissions",
			},
			"internal_tracker": &graphql.Field{
				Type:        internalTracker,
				Description: "Repository permissions",
			},
			"external_tracker": &graphql.Field{
				Type:        externalTracker,
				Description: "Repository permissions",
			},
			"external_wiki": &graphql.Field{
				Type:        externalWiki,
				Description: "Repository permissions",
			},
		},
	},
)

//branch describes a branch
var branch = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "branch",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type: graphql.String,
				Description: "name of the branch",
			},
			"commit": &graphql.Field{
				Type: payloadCommit,
				Description: "",
			},
			"protected": &graphql.Field{
				Type: graphql.Boolean,
				Description: "is branch protection enabled",
			},
			"required_approvals": &graphql.Field{
				Type: gqlInt64,
				Description: "number of approvals required before a pull request can be merged",
			},
			"enable_status_check": &graphql.Field{
				Type: graphql.Boolean,
				Description: "Status checks required before merge enabled",
			},
			"status_check_contexts": &graphql.Field{
				Type: graphql.NewList(graphql.String),
				Description: "List of status check contexts",
			},
			"user_can_push": &graphql.Field{
				Type: graphql.String,
				Description: "Anyone with write access will be allowed to push",
			},
			"user_can_merge": &graphql.Field{
				Type: graphql.String,
				Description: "Anyone with write access will be allowed to merge",
			},
			"effective_branch_protection_name": &graphql.Field{
				Type: graphql.String,
				Description: "The effective branch protection name",
			},
		},
	},
)

// internalTracker represents settings for internal tracker
var internalTracker = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "internal_tracker",
		Fields: graphql.Fields{
			"enable_time_tracker": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "Enable time tracking (Built-in issue tracker)",
			},
			"allow_only_contributors_to_track_time": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "Let only contributors track time",
			},
			"enable_issue_dependencies": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "Enable dependencies for issues and pull requests",
			},
		},
	},
)

// externalTracker represents settings for external tracker
var externalTracker = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "external_tracker",
		Fields: graphql.Fields{
			"external_tracker_url": &graphql.Field{
				Type:        graphql.String,
				Description: "URL of external issue tracker.",
			},
			"external_tracker_format": &graphql.Field{
				Type:        graphql.String,
				Description: "External Issue Tracker URL Format. Use the placeholders {user}, {repo} and {index} for the username, repository name and issue index.",
			},
			"external_tracker_style": &graphql.Field{
				Type:        graphql.String,
				Description: "External Issue Tracker Number Format, either `numeric` or `alphanumeric`",
			},
		},
	},
)

// externalWiki represents setting for external wiki
var externalWiki = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "external_wiki",
		Fields: graphql.Fields{
			"external_wiki_url": &graphql.Field{
				Type:        graphql.String,
				Description: "URL of external wiki",
			},
		},
	},
)

// permission describes a permission
var permission = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "permission",
		Fields: graphql.Fields{
			"admin": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "is admin",
			},
			"push": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "push access",
			},
			"pull": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "pull access",
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
				Type:        gqlInt64,
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

// payloadCommit represents a commit
var payloadCommit = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "payload_commit",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type:        graphql.String,
				Description: "sha1 hash of the commit",
			},
			"message": &graphql.Field{
				Type:        graphql.String,
				Description: "",
			},
			"url": &graphql.Field{
				Type:        graphql.String,
				Description: "",
			},
			"author": &graphql.Field{
				Type:        payloadUser,
				Description: "",
			},
			"committer": &graphql.Field{
				Type:        payloadUser,
				Description: "",
			},
			"verification": &graphql.Field{
				Type:        payloadCommitVerification,
				Description: "",
			},
			"timestamp": &graphql.Field{
				Type:        graphql.DateTime,
				Description: "",
			},
			"added": &graphql.Field{
				Type:        graphql.NewList(graphql.String),
				Description: "",
			},
			"removed": &graphql.Field{
				Type:        graphql.NewList(graphql.String),
				Description: "",
			},
			"modified": &graphql.Field{
				Type:        graphql.NewList(graphql.String),
				Description: "",
			},
		},
	},
)

// payloadUser represents the author or committer of a commit
var payloadUser = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "payload_user",
		Fields: graphql.Fields{
			"name": &graphql.Field{
				Type:        graphql.String,
				Description: "Full name of the commit author",
			},
			"email": &graphql.Field{
				Type:        graphql.String,
				Description: "Email of the commit author",
			},
			"username": &graphql.Field{
				Type:        graphql.String,
				Description: "User name of the commit author",
			},
		},
	},
)

// payloadCommitVerification represents the GPG verification of a commit
var payloadCommitVerification = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "payload_commit_verification",
		Fields: graphql.Fields{
			"verified": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "",
			},
			"reason": &graphql.Field{
				Type:        graphql.String,
				Description: "",
			},
			"signature": &graphql.Field{
				Type:        graphql.String,
				Description: "",
			},
			"signer": &graphql.Field{
				Type:        payloadUser,
				Description: "",
			},
			"payload": &graphql.Field{
				Type:        graphql.String,
				Description: "",
			},
		},
	},
)

// gqlInt64 wraps an int64 because 64-bit int is not directly supported in graphql
var gqlInt64 = graphql.NewScalar(graphql.ScalarConfig{
	Name:        "gqlInt64",
	Description: "The `gqlInt64` scalar type represents an int64 value.",
	// Serialize serializes `gqlInt64` to int64.
	Serialize: func(value interface{}) interface{} {
		switch value := value.(type) {
		case int64:
			return value
		case *int64:
			return *value
		default:
			return nil
		}
	},
	// ParseValue parses GraphQL variables from `int64` to `gqlInt64`.
	ParseValue: func(value interface{}) interface{} {
		switch value := value.(type) {
		case int64:
			return value
		case *int64:
			return *value
		default:
			return nil
		}
	},
	// ParseLiteral parses GraphQL AST value to `gqlInt64`.
	ParseLiteral: func(valueAST ast.Value) interface{} {
		switch valueAST := valueAST.(type) {
		case *ast.StringValue:
			v, err := strconv.ParseInt(valueAST.Value, 10, 64)
			if err != nil {
				return v
			} else {
				return nil
			}
		default:
			return nil
		}
	},
})

func init() {
	//direct circular references not allowed, so adding here as a workaround
	//reference: https://github.com/graphql-go/graphql/issues/164
	generalInfo.AddFieldConfig("parent", &graphql.Field{
		Type:        repository,
		Description: "Parent repository",
	})
}
