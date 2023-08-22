// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"net/http"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/perm"
	access_model "code.gitea.io/gitea/models/perm/access"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	repo_service "code.gitea.io/gitea/services/repository"
	wiki_service "code.gitea.io/gitea/services/wiki"
)

// ServNoCommand returns information about the provided keyid
func ServNoCommand(ctx *context.PrivateContext) {
	keyID := ctx.ParamsInt64(":keyid")
	if keyID <= 0 {
		ctx.JSON(http.StatusBadRequest, private.Response{
			UserMsg: fmt.Sprintf("Bad key id: %d", keyID),
		})
	}
	results := private.KeyAndOwner{}

	key, err := asymkey_model.GetPublicKeyByID(keyID)
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.JSON(http.StatusUnauthorized, private.Response{
				UserMsg: fmt.Sprintf("Cannot find key: %d", keyID),
			})
			return
		}
		log.Error("Unable to get public key: %d Error: %v", keyID, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}
	results.Key = key

	if key.Type == asymkey_model.KeyTypeUser || key.Type == asymkey_model.KeyTypePrincipal {
		user, err := user_model.GetUserByID(ctx, key.OwnerID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.JSON(http.StatusUnauthorized, private.Response{
					UserMsg: fmt.Sprintf("Cannot find owner with id: %d for key: %d", key.OwnerID, keyID),
				})
				return
			}
			log.Error("Unable to get owner with id: %d for public key: %d Error: %v", key.OwnerID, keyID, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err.Error(),
			})
			return
		}
		if !user.IsActive || user.ProhibitLogin {
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: "Your account is disabled.",
			})
			return
		}
		results.Owner = user
	}
	ctx.JSON(http.StatusOK, &results)
}

// ServCommand returns information about the provided keyid
func ServCommand(ctx *context.PrivateContext) {
	keyID := ctx.ParamsInt64(":keyid")
	ownerName := ctx.Params(":owner")
	repoName := ctx.Params(":repo")
	mode := perm.AccessMode(ctx.FormInt("mode"))

	// Set the basic parts of the results to return
	results := private.ServCommandResults{
		RepoName:  repoName,
		OwnerName: ownerName,
		KeyID:     keyID,
	}

	// Now because we're not translating things properly let's just default some English strings here
	modeString := "read"
	if mode > perm.AccessModeRead {
		modeString = "write to"
	}

	// The default unit we're trying to look at is code
	unitType := unit.TypeCode

	// Unless we're a wiki...
	if strings.HasSuffix(repoName, ".wiki") {
		// in which case we need to look at the wiki
		unitType = unit.TypeWiki
		// And we'd better munge the reponame and tell downstream we're looking at a wiki
		results.IsWiki = true
		results.RepoName = repoName[:len(repoName)-5]
	}

	owner, err := user_model.GetUserByName(ctx, results.OwnerName)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			// User is fetching/cloning a non-existent repository
			log.Warn("Failed authentication attempt (cannot find repository: %s/%s) from %s", results.OwnerName, results.RepoName, ctx.RemoteAddr())
			ctx.JSON(http.StatusNotFound, private.Response{
				UserMsg: fmt.Sprintf("Cannot find repository: %s/%s", results.OwnerName, results.RepoName),
			})
			return
		}
		log.Error("Unable to get repository owner: %s/%s Error: %v", results.OwnerName, results.RepoName, err)
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: fmt.Sprintf("Unable to get repository owner: %s/%s %v", results.OwnerName, results.RepoName, err),
		})
		return
	}
	if !owner.IsOrganization() && !owner.IsActive {
		ctx.JSON(http.StatusForbidden, private.Response{
			UserMsg: "Repository cannot be accessed, you could retry it later",
		})
		return
	}

	// Now get the Repository and set the results section
	repoExist := true
	repo, err := repo_model.GetRepositoryByName(owner.ID, results.RepoName)
	if err != nil {
		if repo_model.IsErrRepoNotExist(err) {
			repoExist = false
			for _, verb := range ctx.FormStrings("verb") {
				if verb == "git-upload-pack" {
					// User is fetching/cloning a non-existent repository
					log.Warn("Failed authentication attempt (cannot find repository: %s/%s) from %s", results.OwnerName, results.RepoName, ctx.RemoteAddr())
					ctx.JSON(http.StatusNotFound, private.Response{
						UserMsg: fmt.Sprintf("Cannot find repository: %s/%s", results.OwnerName, results.RepoName),
					})
					return
				}
			}
		} else {
			log.Error("Unable to get repository: %s/%s Error: %v", results.OwnerName, results.RepoName, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get repository: %s/%s %v", results.OwnerName, results.RepoName, err),
			})
			return
		}
	}

	if repoExist {
		repo.Owner = owner
		repo.OwnerName = ownerName
		results.RepoID = repo.ID

		if repo.IsBeingCreated() {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: "Repository is being created, you could retry after it finished",
			})
			return
		}

		if repo.IsBroken() {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: "Repository is in a broken state",
			})
			return
		}

		// We can shortcut at this point if the repo is a mirror
		if mode > perm.AccessModeRead && repo.IsMirror {
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: fmt.Sprintf("Mirror Repository %s/%s is read-only", results.OwnerName, results.RepoName),
			})
			return
		}
	}

	// Get the Public Key represented by the keyID
	key, err := asymkey_model.GetPublicKeyByID(keyID)
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.JSON(http.StatusNotFound, private.Response{
				UserMsg: fmt.Sprintf("Cannot find key: %d", keyID),
			})
			return
		}
		log.Error("Unable to get public key: %d Error: %v", keyID, err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: fmt.Sprintf("Unable to get key: %d  Error: %v", keyID, err),
		})
		return
	}
	results.KeyName = key.Name
	results.KeyID = key.ID
	results.UserID = key.OwnerID

	// If repo doesn't exist, deploy key doesn't make sense
	if !repoExist && key.Type == asymkey_model.KeyTypeDeploy {
		ctx.JSON(http.StatusNotFound, private.Response{
			UserMsg: fmt.Sprintf("Cannot find repository %s/%s", results.OwnerName, results.RepoName),
		})
		return
	}

	// Deploy Keys have ownerID set to 0 therefore we can't use the owner
	// So now we need to check if the key is a deploy key
	// We'll keep hold of the deploy key here for permissions checking
	var deployKey *asymkey_model.DeployKey
	var user *user_model.User
	if key.Type == asymkey_model.KeyTypeDeploy {
		var err error
		deployKey, err = asymkey_model.GetDeployKeyByRepo(ctx, key.ID, repo.ID)
		if err != nil {
			if asymkey_model.IsErrDeployKeyNotExist(err) {
				ctx.JSON(http.StatusNotFound, private.Response{
					UserMsg: fmt.Sprintf("Public (Deploy) Key: %d:%s is not authorized to %s %s/%s.", key.ID, key.Name, modeString, results.OwnerName, results.RepoName),
				})
				return
			}
			log.Error("Unable to get deploy for public (deploy) key: %d in %-v Error: %v", key.ID, repo, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get Deploy Key for Public Key: %d:%s in %s/%s.", key.ID, key.Name, results.OwnerName, results.RepoName),
			})
			return
		}
		results.DeployKeyID = deployKey.ID
		results.KeyName = deployKey.Name

		// FIXME: Deploy keys aren't really the owner of the repo pushing changes
		// however we don't have good way of representing deploy keys in hook.go
		// so for now use the owner of the repository
		results.UserName = results.OwnerName
		results.UserID = repo.OwnerID
		if !repo.Owner.KeepEmailPrivate {
			results.UserEmail = repo.Owner.Email
		}
	} else {
		// Get the user represented by the Key
		var err error
		user, err = user_model.GetUserByID(ctx, key.OwnerID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.JSON(http.StatusUnauthorized, private.Response{
					UserMsg: fmt.Sprintf("Public Key: %d:%s owner %d does not exist.", key.ID, key.Name, key.OwnerID),
				})
				return
			}
			log.Error("Unable to get owner: %d for public key: %d:%s Error: %v", key.OwnerID, key.ID, key.Name, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get Owner: %d for Deploy Key: %d:%s in %s/%s.", key.OwnerID, key.ID, key.Name, ownerName, repoName),
			})
			return
		}

		if !user.IsActive || user.ProhibitLogin {
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: "Your account is disabled.",
			})
			return
		}

		results.UserName = user.Name
		if !user.KeepEmailPrivate {
			results.UserEmail = user.Email
		}
	}

	// Don't allow pushing if the repo is archived
	if repoExist && mode > perm.AccessModeRead && repo.IsArchived {
		ctx.JSON(http.StatusUnauthorized, private.Response{
			UserMsg: fmt.Sprintf("Repo: %s/%s is archived.", results.OwnerName, results.RepoName),
		})
		return
	}

	// Permissions checking:
	if repoExist &&
		(mode > perm.AccessModeRead ||
			repo.IsPrivate ||
			owner.Visibility.IsPrivate() ||
			(user != nil && user.IsRestricted) || // user will be nil if the key is a deploykey
			setting.Service.RequireSignInView) {
		if key.Type == asymkey_model.KeyTypeDeploy {
			if deployKey.Mode < mode {
				ctx.JSON(http.StatusUnauthorized, private.Response{
					UserMsg: fmt.Sprintf("Deploy Key: %d:%s is not authorized to %s %s/%s.", key.ID, key.Name, modeString, results.OwnerName, results.RepoName),
				})
				return
			}
		} else {
			// Because of the special ref "refs/for" we will need to delay write permission check
			if git.SupportProcReceive && unitType == unit.TypeCode {
				mode = perm.AccessModeRead
			}

			perm, err := access_model.GetUserRepoPermission(ctx, repo, user)
			if err != nil {
				log.Error("Unable to get permissions for %-v with key %d in %-v Error: %v", user, key.ID, repo, err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: fmt.Sprintf("Unable to get permissions for user %d:%s with key %d in %s/%s Error: %v", user.ID, user.Name, key.ID, results.OwnerName, results.RepoName, err),
				})
				return
			}

			userMode := perm.UnitAccessMode(unitType)

			if userMode < mode {
				log.Warn("Failed authentication attempt for %s with key %s (not authorized to %s %s/%s) from %s", user.Name, key.Name, modeString, ownerName, repoName, ctx.RemoteAddr())
				ctx.JSON(http.StatusUnauthorized, private.Response{
					UserMsg: fmt.Sprintf("User: %d:%s with Key: %d:%s is not authorized to %s %s/%s.", user.ID, user.Name, key.ID, key.Name, modeString, ownerName, repoName),
				})
				return
			}
		}
	}

	// We already know we aren't using a deploy key
	if !repoExist {
		owner, err := user_model.GetUserByName(ctx, ownerName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Unable to get owner: %s %v", results.OwnerName, err),
			})
			return
		}

		if owner.IsOrganization() && !setting.Repository.EnablePushCreateOrg {
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: "Push to create is not enabled for organizations.",
			})
			return
		}
		if !owner.IsOrganization() && !setting.Repository.EnablePushCreateUser {
			ctx.JSON(http.StatusForbidden, private.Response{
				UserMsg: "Push to create is not enabled for users.",
			})
			return
		}

		repo, err = repo_service.PushCreateRepo(ctx, user, owner, results.RepoName)
		if err != nil {
			log.Error("pushCreateRepo: %v", err)
			ctx.JSON(http.StatusNotFound, private.Response{
				UserMsg: fmt.Sprintf("Cannot find repository: %s/%s", results.OwnerName, results.RepoName),
			})
			return
		}
		results.RepoID = repo.ID
	}

	if results.IsWiki {
		// Ensure the wiki is enabled before we allow access to it
		if _, err := repo.GetUnit(ctx, unit.TypeWiki); err != nil {
			if repo_model.IsErrUnitTypeNotExist(err) {
				ctx.JSON(http.StatusForbidden, private.Response{
					UserMsg: "repository wiki is disabled",
				})
				return
			}
			log.Error("Failed to get the wiki unit in %-v Error: %v", repo, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Failed to get the wiki unit in %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}

		// Finally if we're trying to touch the wiki we should init it
		if err = wiki_service.InitWiki(ctx, repo); err != nil {
			log.Error("Failed to initialize the wiki in %-v Error: %v", repo, err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: fmt.Sprintf("Failed to initialize the wiki in %s/%s Error: %v", ownerName, repoName, err),
			})
			return
		}
	}
	log.Debug("Serv Results:\nIsWiki: %t\nDeployKeyID: %d\nKeyID: %d\tKeyName: %s\nUserName: %s\nUserID: %d\nOwnerName: %s\nRepoName: %s\nRepoID: %d",
		results.IsWiki,
		results.DeployKeyID,
		results.KeyID,
		results.KeyName,
		results.UserName,
		results.UserID,
		results.OwnerName,
		results.RepoName,
		results.RepoID)

	ctx.JSON(http.StatusOK, results)
	// We will update the keys in a different call.
}
