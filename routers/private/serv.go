// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"net/http"
	"strings"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/git"
	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"
	"gitea.dev/modules/private"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
	"gitea.dev/services/context"
	repo_service "gitea.dev/services/repository"
	wiki_service "gitea.dev/services/wiki"
)

// ServNoCommand returns information about the provided keyid
func ServNoCommand(ctx *context.PrivateContext) {
	keyID := ctx.PathParamInt64("keyid")
	if keyID <= 0 {
		ctx.PrivateUserErrorf(http.StatusBadRequest, "Bad key id: %d", keyID)
		return
	}
	results := private.KeyAndOwner{}

	key, err := asymkey_model.GetPublicKeyByID(ctx, keyID)
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.PrivateUserErrorf(http.StatusUnauthorized, "Cannot find key: %d", keyID)
			return
		}
		ctx.PrivateInternalErrorf("Unable to get public key: %d Error: %v", keyID, err)
		return
	}
	results.Key = key

	if key.Type == asymkey_model.KeyTypeUser || key.Type == asymkey_model.KeyTypePrincipal {
		user, err := user_model.GetUserByID(ctx, key.OwnerID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.PrivateUserErrorf(http.StatusUnauthorized, "Cannot find owner with id: %d for key: %d", key.OwnerID, keyID)
				return
			}
			ctx.PrivateInternalErrorf("Unable to get owner with id: %d for public key: %d Error: %v", key.OwnerID, keyID, err)
			return
		}
		if !user.IsActive || user.ProhibitLogin {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Your account is disabled.")
			return
		}
		results.Owner = user
	}
	ctx.JSON(http.StatusOK, &results)
}

// ServCommand returns information about the provided keyid
func ServCommand(ctx *context.PrivateContext) {
	keyID := ctx.PathParamInt64("keyid")
	reqOwnerName := ctx.PathParam("owner")
	reqRepoName := ctx.PathParam("repo")
	mode := perm.AccessMode(ctx.FormInt("mode"))
	verb := ctx.FormString("verb")

	// Set the basic parts of the results to return
	results := private.ServCommandResults{
		OwnerName: reqOwnerName, // it might be changed if there is "renamed user redirection"
		RepoName:  reqRepoName,  // it might be changed if there is "renamed repo redirection", or the repo is a wiki
		KeyID:     keyID,
	}
	repoLogName := reqOwnerName + "/" + reqRepoName

	if reqWikiRepoName, ok := strings.CutSuffix(reqRepoName, ".wiki"); ok {
		// in which case we need to look at the wiki, trim the ".wiki" suffix, only use the main repo name
		results.IsWiki = true
		results.RepoName = reqWikiRepoName
	}
	unitType := util.Iif(results.IsWiki, unit.TypeWiki, unit.TypeCode)
	modeString := mode.ToString()

	owner, err := user_model.GetUserByName(ctx, results.OwnerName)
	if err != nil {
		if !user_model.IsErrUserNotExist(err) {
			ctx.PrivateInternalErrorf("Unable to get repository owner for %s, error: %v", repoLogName, err)
			return
		}

		// Check if there is a user redirect for the requested owner
		redirectedUserID, err := user_model.LookupUserRedirect(ctx, results.OwnerName)
		if err != nil {
			ctx.PrivateUserErrorf(http.StatusNotFound, "Cannot find repository %s", repoLogName)
			return
		}
		redirectUser, err := user_model.GetUserByID(ctx, redirectedUserID)
		if err != nil {
			ctx.PrivateUserErrorf(http.StatusNotFound, "Cannot find repository: %s", repoLogName)
			return
		}

		log.Debug("User %s has been redirected to %s", results.OwnerName, redirectUser.Name)
		results.OwnerName = redirectUser.Name
		owner = redirectUser
	}

	// Now get the Repository and set the results section
	repo, err := repo_model.GetRepositoryByName(ctx, owner.ID, results.RepoName)
	if err != nil {
		if !repo_model.IsErrRepoNotExist(err) {
			ctx.PrivateInternalErrorf("Unable to get repository %s, error: %v", repoLogName, err)
			return
		}

		redirectedRepoID, err := repo_model.LookupRedirect(ctx, owner.ID, results.RepoName)
		if err == nil {
			redirectedRepo, err := repo_model.GetRepositoryByID(ctx, redirectedRepoID)
			if err == nil {
				log.Info("Repository %s has been redirected to %s/%s", repoLogName, redirectedRepo.OwnerName, redirectedRepo.Name)
				repo = redirectedRepo
				if err = repo.LoadOwner(ctx); err != nil {
					ctx.PrivateInternalErrorf("Unable to repository owner %d", repo.OwnerID)
					return
				}
				owner = repo.Owner
				results.RepoName = redirectedRepo.Name
				results.OwnerName = redirectedRepo.OwnerName
				repoLogName = results.OwnerName + "/" + results.RepoName + util.Iif(results.IsWiki, ".wiki", "")
			} else {
				log.Warn("Repo %s has a redirect to repo with ID %d, but no repo with this ID could be found. Trying without redirect...", repoLogName, redirectedRepoID)
			}
		}

		if repo == nil {
			if mode == perm.AccessModeRead {
				// User is fetching/cloning a non-existent repository
				ctx.PrivateUserErrorf(http.StatusNotFound, "Cannot find repository %s", repoLogName)
				return
			}
		}
	}

	if !owner.IsOrganization() && !owner.IsActive {
		ctx.PrivateUserErrorf(http.StatusForbidden, "Repository cannot be accessed, the owner is inactive.")
		return
	}

	if repo != nil {
		repo.Owner = owner
		repo.OwnerName = owner.Name
		results.RepoID = repo.ID

		if repo.IsBeingCreated() {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Repository is being created, you could retry after it finished")
			return
		}

		if repo.IsBroken() {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Repository is in a broken state")
			return
		}

		// We can shortcut at this point if the repo is a mirror
		if mode > perm.AccessModeRead && repo.IsMirror {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Mirror Repository %s is read-only", repoLogName)
			return
		}
	}

	// Get the Public Key represented by the keyID
	key, err := asymkey_model.GetPublicKeyByID(ctx, keyID)
	if err != nil {
		if asymkey_model.IsErrKeyNotExist(err) {
			ctx.PrivateUserErrorf(http.StatusNotFound, "Cannot find key: %d", keyID)
			return
		}
		ctx.PrivateInternalErrorf("Unable to get key: %d, error: %v", keyID, err)
		return
	}
	results.KeyName = key.Name
	results.KeyID = key.ID
	results.UserID = key.OwnerID

	// Deploy Keys have ownerID set to 0 therefore we can't use the owner
	// So now we need to check if the key is a deploy key
	// We'll keep hold of the deploy key here for permissions checking
	var deployKey *asymkey_model.DeployKey
	var user *user_model.User
	if key.Type == asymkey_model.KeyTypeDeploy {
		if repo == nil {
			ctx.PrivateUserErrorf(http.StatusNotFound, "Cannot find repository %s", repoLogName)
			return
		}
		deployKey, err = asymkey_model.GetDeployKeyByRepo(ctx, key.ID, repo.ID)
		if err != nil {
			if asymkey_model.IsErrDeployKeyNotExist(err) {
				ctx.PrivateUserErrorf(http.StatusNotFound, "Deploy key %d:%s has no %q permission for %s.", key.ID, key.Name, modeString, repoLogName)
				return
			}
			ctx.PrivateInternalErrorf("Unable to get deploy for public (deploy) key %d for %s, error: %v", key.ID, repoLogName, err)
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
		user, err = user_model.GetUserByID(ctx, key.OwnerID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				ctx.PrivateUserErrorf(http.StatusUnauthorized, "Public key %d:%s owner %d does not exist.", key.ID, key.Name, key.OwnerID)
				return
			}
			ctx.PrivateInternalErrorf("Unable to get key owner %d for public key %d:%s, error: %v", key.OwnerID, key.ID, key.Name, err)
			return
		}

		if !user.IsActive || user.ProhibitLogin {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Your account is disabled.")
			return
		}

		results.UserName = user.Name
		if !user.KeepEmailPrivate {
			results.UserEmail = user.Email
		}
	}

	// Don't allow pushing if the repo is archived
	if repo != nil && mode > perm.AccessModeRead && repo.IsArchived {
		ctx.PrivateUserErrorf(http.StatusUnauthorized, "Repo %s is archived.", repoLogName)
		return
	}

	// Permissions checking:
	if repo != nil &&
		(mode > perm.AccessModeRead ||
			repo.IsPrivate ||
			owner.Visibility.IsPrivate() ||
			(user != nil && user.IsRestricted) || // user will be nil if the key is a deploy key
			setting.Service.RequireSignInViewStrict) {
		if key.Type == asymkey_model.KeyTypeDeploy {
			if deployKey == nil || deployKey.Mode < mode {
				ctx.PrivateUserErrorf(http.StatusUnauthorized, "Deploy key %d:%s has no %q permission for %s.", key.ID, key.Name, modeString, repoLogName)
				return
			}
		} else {
			// Because of the special ref "refs/for" (AGit) we will need to delay write permission check,
			// AGit flow needs to write its own ref when the doer has "reader" permission (allowing to create PR).
			// The real permission check is done in HookPreReceive (routers/private/hook_pre_receive.go).
			// Here it should relax the permission check for "git push (git-receive-pack)", but not for others like LFS operations.
			if git.DefaultFeatures().SupportProcReceive && unitType == unit.TypeCode && verb == git.CmdVerbReceivePack {
				mode = perm.AccessModeRead
			}

			userPerm, err := access_model.GetDoerRepoPermission(ctx, repo, user)
			if err != nil {
				ctx.PrivateInternalErrorf("Unable to get permissions for %-v with key %d in %-v, error: %v", user, key.ID, repo, err)
				return
			}

			userMode := userPerm.UnitAccessMode(unitType)
			if userMode < mode {
				ctx.PrivateUserErrorf(http.StatusUnauthorized, "User %d with key %d:%s has no %q permission for %s", key.OwnerID, key.ID, key.Name, modeString, repoLogName)
				return
			}
		}
	}

	// We already know we aren't using a deploy key
	if repo == nil {
		if owner.IsOrganization() && !setting.Repository.EnablePushCreateOrg {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Push to create is not enabled for organizations.")
			return
		}
		if !owner.IsOrganization() && !setting.Repository.EnablePushCreateUser {
			ctx.PrivateUserErrorf(http.StatusForbidden, "Push to create is not enabled for users.")
			return
		}

		repo, err = repo_service.PushCreateRepo(ctx, user, owner, results.RepoName)
		if err != nil {
			ctx.PrivateInternalErrorf("pushCreateRepo: %v", err)
			return
		}
		results.RepoID = repo.ID
	}

	if results.IsWiki {
		// Ensure the wiki is enabled before we allow access to it
		if _, err := repo.GetUnit(ctx, unit.TypeWiki); err != nil {
			if repo_model.IsErrUnitTypeNotExist(err) {
				ctx.PrivateUserErrorf(http.StatusForbidden, "repository wiki is disabled")
				return
			}
			ctx.PrivateInternalErrorf("Failed to get the wiki unit in %-v, error: %v", repo, err)
			return
		}

		// Finally if we're trying to touch the wiki we should init it
		if err = wiki_service.InitWiki(ctx, repo); err != nil {
			ctx.PrivateInternalErrorf("Failed to initialize the wiki in %-v, error: %v", repo, err)
			return
		}
	}

	gitRepo := util.Iif(results.IsWiki, repo.WikiStorageRepo(), repo.CodeStorageRepo())
	results.RepoStoragePath = gitrepo.RepoLocalPath(gitRepo)
	log.Debug("Serv Results: %+v", results)
	ctx.JSON(http.StatusOK, results)
	// We will update the keys in a different call.
}
