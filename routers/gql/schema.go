// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gql

import (
	"context"
	"errors"
	"strconv"

	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/seripap/relay"
)

// nodeDefinitions functions to allow an arbitrary type (node) to be looked up by its id
var nodeDefinitions *relay.NodeDefinitions
var repository *graphql.Object
var branch *graphql.Object
var internalTracker *graphql.Object
var externalTracker *graphql.Object
var externalWiki *graphql.Object
var permission *graphql.Object
var user *graphql.Object
var payloadCommit *graphql.Object
var payloadUser *graphql.Object
var payloadCommitVerification *graphql.Object

// Schema the graphqa schema
var Schema graphql.Schema

func init() {
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
				}
				return nil
			default:
				return nil
			}
		},
	})

	nodeDefinitions = relay.NewNodeDefinitions(relay.NodeDefinitionsConfig{
		IDFetcher: func(id string, info graphql.ResolveInfo, ctx context.Context) (interface{}, error) {
			resolvedID := relay.FromGlobalID(id)
			// based on id and its type, return the object
			switch resolvedID.Type {
			case "repository":
				return RepositoryByIDResolver(ctx, resolvedID.ID)
			case "user":
				return UserByIDResolver(ctx, resolvedID.ID)
			default:
				return nil, errors.New("Unknown node type")
			}
		},
		TypeResolve: func(p graphql.ResolveTypeParams) *graphql.Object {
			// based on the type of the value, return GraphQLObjectType
			switch p.Value.(type) {
			case *api.GqlRepository:
				return repository
			case *api.User:
				return user
			default:
				return repository
			}
		},
	})

	//user describes a user
	user = graphql.NewObject(
		graphql.ObjectConfig{
			Name:        "user",
			Description: "A user",
			Fields: graphql.Fields{
				"id": relay.GlobalIDField("user", nil),
				"rest_api_id": &graphql.Field{
					Type:        gqlInt64,
					Description: "Id from REST API",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source.(*api.User).ID, nil
					},
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
			Interfaces: []*graphql.Interface{
				nodeDefinitions.NodeInterface,
			},
		},
	)

	var userConnectionDefinition = relay.ConnectionDefinitions(relay.ConnectionConfig{
		Name:     "user",
		NodeType: user,
	})

	// internalTracker represents settings for internal tracker
	internalTracker = graphql.NewObject(
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
	externalTracker = graphql.NewObject(
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
	externalWiki = graphql.NewObject(
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
	permission = graphql.NewObject(
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

	// payloadUser represents the author or committer of a commit
	payloadUser = graphql.NewObject(
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
	payloadCommitVerification = graphql.NewObject(
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

	// payloadCommit represents a commit
	payloadCommit = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "payload_commit",
			Fields: graphql.Fields{
				"id": &graphql.Field{
					Type:        graphql.String,
					Description: "sha1 hash of the commit",
				},
				"message": &graphql.Field{
					Type:        graphql.String,
					Description: "Commit message",
				},
				"url": &graphql.Field{
					Type:        graphql.String,
					Description: "",
				},
				"author": &graphql.Field{
					Type:        payloadUser,
					Description: "Commit author",
				},
				"committer": &graphql.Field{
					Type:        payloadUser,
					Description: "",
				},
				"verification": &graphql.Field{
					Type:        payloadCommitVerification,
					Description: "The GPG verification of a commit",
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

	//branch a git branch on a repository
	branch = graphql.NewObject(
		graphql.ObjectConfig{
			Name:        "branch",
			Description: "A git branch on a repository",
			Fields: graphql.Fields{
				"name": &graphql.Field{
					Type:        graphql.String,
					Description: "name of the branch",
				},
				"commit": &graphql.Field{
					Type:        payloadCommit,
					Description: "",
				},
				"protected": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "is branch protection enabled",
				},
				"required_approvals": &graphql.Field{
					Type:        gqlInt64,
					Description: "number of approvals required before a pull request can be merged",
				},
				"enable_status_check": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "status checks required before merge enabled",
				},
				"status_check_contexts": &graphql.Field{
					Type:        graphql.NewList(graphql.String),
					Description: "list of status check contexts",
				},
				"user_can_push": &graphql.Field{
					Type:        graphql.String,
					Description: "anyone with write access will be allowed to push",
				},
				"user_can_merge": &graphql.Field{
					Type:        graphql.String,
					Description: "anyone with write access will be allowed to merge",
				},
				"effective_branch_protection_name": &graphql.Field{
					Type:        graphql.String,
					Description: "the effective branch protection name",
				},
			},
		},
	)

	// repository a gitea repository
	repository = graphql.NewObject(
		graphql.ObjectConfig{
			Name:        "repository",
			Description: "A Gitea repository",
			Fields: graphql.Fields{
				"id": relay.GlobalIDField("repository", nil),
				"rest_api_id": &graphql.Field{
					Type:        gqlInt64,
					Description: "Id from REST API",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						return p.Source.(api.GqlRepository).ID, nil
					},
				},
				"owner": &graphql.Field{
					Type:        user,
					Description: "Owner of the repository",
				},
				"name": &graphql.Field{
					Type:        graphql.String,
					Description: "Name of the repository",
				},
				"full_name": &graphql.Field{
					Type:        graphql.String,
					Description: "Full name of the repository",
				},
				"description": &graphql.Field{
					Type:        graphql.String,
					Description: "Description of the repository",
				},
				"empty": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Whether the repository is empty or not.",
				},
				"private": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Whether the repository is private or not",
				},
				"fork": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Whether the repository is a fork or not",
				},
				"template": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is this repository a template",
				},
				"mirror": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is this repository a mirror",
				},
				"archived": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is this repository archived",
				},
				"has_issues": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Does this repository contain issues",
				},
				"has_wiki": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Does this repository have a wiki",
				},
				"has_pull_requests": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Does this repository have pull requests",
				},
				"ignore_whitespace_conflicts": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Does this repository ignore whitespace for conflicts",
				},
				"allow_merge_commits": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is commit merging enabled",
				},
				"allow_rebase": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is rebasing enabled",
				},
				"allow_rebase_explicit": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is rebasing with explicit merge commits (--no-ff) enabled",
				},
				"allow_squash_merge": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is squashing to merge commits enabled",
				},
				"internal": &graphql.Field{
					Type:        graphql.Boolean,
					Description: "Is visibility of repository set to private",
				},
				"size": &graphql.Field{
					Type:        graphql.Int,
					Description: "Repository size",
				},
				"stars_count": &graphql.Field{
					Type:        graphql.Int,
					Description: "Number of stars",
				},
				"forks_count": &graphql.Field{
					Type:        graphql.Int,
					Description: "Number of forks",
				},
				"watchers_count": &graphql.Field{
					Type:        graphql.Int,
					Description: "Number of watchers",
				},
				"open_issues_count": &graphql.Field{
					Type:        graphql.Int,
					Description: "Number of open issues",
				},
				"open_pr_counter": &graphql.Field{
					Type:        graphql.Int,
					Description: "Number of open pull requests",
				},
				"release_counter": &graphql.Field{
					Type:        graphql.Int,
					Description: "Number of releases",
				},
				"html_url": &graphql.Field{
					Type:        graphql.String,
					Description: "HTML url of repository",
				},
				"ssh_url": &graphql.Field{
					Type:        graphql.String,
					Description: "SSH url of repository",
				},
				"clone_url": &graphql.Field{
					Type:        graphql.String,
					Description: "Clone url of repository",
				},
				"website": &graphql.Field{
					Type:        graphql.String,
					Description: "The repository's website address",
				},
				"default_branch": &graphql.Field{
					Type:        graphql.String,
					Description: "The default branch",
				},
				"avatar_url": &graphql.Field{
					Type:        graphql.String,
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
				"branches": &graphql.Field{
					Type:        graphql.NewList(branch),
					Description: "Branches contained within a repostory",
					Resolve:     BranchesResolver,
				},
				"collaborators": &graphql.Field{
					Type:        userConnectionDefinition.ConnectionType,
					Args:        relay.ConnectionArgs,
					Description: "The repository's collaborators",
					Resolve:     CollaboratorsResolver,
				},
			},
			Interfaces: []*graphql.Interface{
				nodeDefinitions.NodeInterface,
			},
		},
	)

	//direct circular references during initial definition is not allowed, so adding after the type is defined
	//reference: https://github.com/graphql-go/graphql/issues/164
	repository.AddFieldConfig("parent", &graphql.Field{
		Type:        repository,
		Description: "Parent repository",
	})

	var err error
	rootQuery := NewRoot()
	Schema, err = graphql.NewSchema(
		graphql.SchemaConfig{
			Query: rootQuery.Query,
		},
	)
	if err != nil {
		log.Error("Error creating graphql schema: ", err)
		panic(err)
	}
}
