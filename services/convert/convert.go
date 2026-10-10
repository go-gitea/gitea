// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package convert

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"time"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/auth"
	deploykey_model "gitea.dev/models/deploykey"
	git_model "gitea.dev/models/git"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/util"
	asymkey_service "gitea.dev/services/asymkey"
	"gitea.dev/services/gitdiff"
)

// ToEmail convert models.EmailAddress to api.Email
func ToEmail(email *user_model.EmailAddress) *api.Email {
	return &api.Email{
		Email:    email.Email,
		Verified: email.IsActivated,
		Primary:  email.IsPrimary,
	}
}

// ToEmail convert models.EmailAddress to api.Email
func ToEmailSearch(email *user_model.SearchEmailResult) *api.Email {
	return &api.Email{
		Email:    email.Email,
		Verified: email.IsActivated,
		Primary:  email.IsPrimary,
		UserID:   email.UID,
		UserName: email.Name,
	}
}

// ToBranch convert a git.Commit and git.Branch to an api.Branch
func ToBranch(ctx context.Context, repo *repo_model.Repository, branchName string, c *git.Commit, bp *git_model.ProtectedBranch, user *user_model.User, isRepoAdmin bool) (*api.Branch, error) {
	if bp == nil {
		var hasPerm bool
		var canPush bool
		var err error
		if user != nil {
			hasPerm, err = access_model.HasAccessUnit(ctx, user, repo, unit.TypeCode, perm.AccessModeWrite)
			if err != nil {
				return nil, err
			}

			perms, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
			if err != nil {
				return nil, err
			}
			canPush = issues_model.CanMaintainerWriteToBranch(ctx, perms, branchName, user)
		}

		return &api.Branch{
			Name:                branchName,
			Commit:              ToPayloadCommit(ctx, repo, c),
			Protected:           false,
			RequiredApprovals:   0,
			EnableStatusCheck:   false,
			StatusCheckContexts: []string{},
			UserCanPush:         canPush,
			UserCanMerge:        hasPerm,
		}, nil
	}

	branch := &api.Branch{
		Name:                branchName,
		Commit:              ToPayloadCommit(ctx, repo, c),
		Protected:           true,
		RequiredApprovals:   bp.RequiredApprovals,
		EnableStatusCheck:   bp.EnableStatusCheck,
		StatusCheckContexts: bp.StatusCheckContexts,
	}

	if isRepoAdmin {
		branch.EffectiveBranchProtectionName = bp.RuleName
	}

	if user != nil {
		permission, err := access_model.GetIndividualUserRepoPermission(ctx, repo, user)
		if err != nil {
			return nil, err
		}
		bp.Repo = repo
		branch.UserCanPush = bp.CanUserPush(ctx, user, permission)
		branch.UserCanMerge = git_model.IsUserMergeWhitelisted(ctx, bp, user.ID, permission)
	}

	return branch, nil
}

// getWhitelistEntities returns the names of the entities that are in the whitelist
func getWhitelistEntities[T *user_model.User | *organization.Team](entities []T, whitelistIDs []int64) []string {
	whitelistUserIDsSet := container.SetOf(whitelistIDs...)
	whitelistNames := make([]string, 0)
	for _, entity := range entities {
		switch v := any(entity).(type) {
		case *user_model.User:
			if whitelistUserIDsSet.Contains(v.ID) {
				whitelistNames = append(whitelistNames, v.Name)
			}
		case *organization.Team:
			if whitelistUserIDsSet.Contains(v.ID) {
				whitelistNames = append(whitelistNames, v.Name)
			}
		}
	}

	return whitelistNames
}

// ToBranchProtection convert a ProtectedBranch to api.BranchProtection
func ToBranchProtection(ctx context.Context, bp *git_model.ProtectedBranch, repo *repo_model.Repository) *api.BranchProtection {
	readers, err := access_model.GetUsersWithAnyUnitAccess(ctx, repo, perm.AccessModeRead, unit.TypeCode, unit.TypePullRequests)
	if err != nil {
		log.Error("GetRepoReaders: %v", err)
	}

	pushWhitelistUsernames := getWhitelistEntities(readers, bp.WhitelistUserIDs)
	forcePushAllowlistUsernames := getWhitelistEntities(readers, bp.ForcePushAllowlistUserIDs)
	mergeWhitelistUsernames := getWhitelistEntities(readers, bp.MergeWhitelistUserIDs)
	approvalsWhitelistUsernames := getWhitelistEntities(readers, bp.ApprovalsWhitelistUserIDs)
	bypassAllowlistUsernames := getWhitelistEntities(readers, bp.BypassAllowlistUserIDs)

	teamReaders, err := organization.GetTeamsWithAccessToAnyRepoUnit(ctx, repo.Owner.ID, repo.ID, perm.AccessModeRead, unit.TypeCode, unit.TypePullRequests)
	if err != nil {
		log.Error("Repo.Owner.TeamsWithAccessToRepo: %v", err)
	}

	pushWhitelistTeams := getWhitelistEntities(teamReaders, bp.WhitelistTeamIDs)
	forcePushAllowlistTeams := getWhitelistEntities(teamReaders, bp.ForcePushAllowlistTeamIDs)
	mergeWhitelistTeams := getWhitelistEntities(teamReaders, bp.MergeWhitelistTeamIDs)
	approvalsWhitelistTeams := getWhitelistEntities(teamReaders, bp.ApprovalsWhitelistTeamIDs)
	bypassAllowlistTeams := getWhitelistEntities(teamReaders, bp.BypassAllowlistTeamIDs)

	branchName := ""
	if !git_model.IsRuleNameSpecial(bp.RuleName) {
		branchName = bp.RuleName
	}

	return &api.BranchProtection{
		BranchName:                    branchName, //nolint:staticcheck // deprecated but useful to API response
		RuleName:                      bp.RuleName,
		Priority:                      bp.Priority,
		EnablePush:                    bp.CanPush,
		EnablePushWhitelist:           bp.EnableWhitelist,
		PushWhitelistUsernames:        pushWhitelistUsernames,
		PushWhitelistTeams:            pushWhitelistTeams,
		PushWhitelistDeployKeys:       bp.WhitelistDeployKeys,
		EnableForcePush:               bp.CanForcePush,
		EnableForcePushAllowlist:      bp.EnableForcePushAllowlist,
		ForcePushAllowlistUsernames:   forcePushAllowlistUsernames,
		ForcePushAllowlistTeams:       forcePushAllowlistTeams,
		ForcePushAllowlistDeployKeys:  bp.ForcePushAllowlistDeployKeys,
		EnableMergeWhitelist:          bp.EnableMergeWhitelist,
		MergeWhitelistUsernames:       mergeWhitelistUsernames,
		MergeWhitelistTeams:           mergeWhitelistTeams,
		EnableBypassAllowlist:         bp.EnableBypassAllowlist,
		BypassAllowlistUsernames:      bypassAllowlistUsernames,
		BypassAllowlistTeams:          bypassAllowlistTeams,
		EnableStatusCheck:             bp.EnableStatusCheck,
		StatusCheckContexts:           bp.StatusCheckContexts,
		RequiredApprovals:             bp.RequiredApprovals,
		EnableApprovalsWhitelist:      bp.EnableApprovalsWhitelist,
		ApprovalsWhitelistUsernames:   approvalsWhitelistUsernames,
		ApprovalsWhitelistTeams:       approvalsWhitelistTeams,
		BlockOnRejectedReviews:        bp.BlockOnRejectedReviews,
		BlockOnOfficialReviewRequests: bp.BlockOnOfficialReviewRequests,
		BlockOnCodeownerReviews:       bp.BlockOnCodeownerReviews,
		BlockOnOutdatedBranch:         bp.BlockOnOutdatedBranch,
		DismissStaleApprovals:         bp.DismissStaleApprovals,
		IgnoreStaleApprovals:          bp.IgnoreStaleApprovals,
		RequireSignedCommits:          bp.RequireSignedCommits,
		ProtectedFilePatterns:         bp.ProtectedFilePatterns,
		UnprotectedFilePatterns:       bp.UnprotectedFilePatterns,
		BlockAdminMergeOverride:       bp.BlockAdminMergeOverride,
		Created:                       bp.CreatedUnix.AsTime(),
		Updated:                       bp.UpdatedUnix.AsTime(),
	}
}

// ToTag convert a git.Tag to an api.Tag
func ToTag(repo *repo_model.Repository, t *git.Tag) *api.Tag {
	tarballURL := repo.HTMLURL() + "/archive/" + url.PathEscape(t.Name+".tar.gz")
	zipballURL := repo.HTMLURL() + "/archive/" + url.PathEscape(t.Name+".zip")

	// Archive URLs are "" if the download feature is disabled
	if setting.Repository.DisableDownloadSourceArchives {
		tarballURL = ""
		zipballURL = ""
	}

	return &api.Tag{
		Name:       t.Name,
		Message:    t.MessageUTF8(),
		ID:         t.ID.String(),
		Commit:     ToCommitMeta(repo, t),
		ZipballURL: zipballURL,
		TarballURL: tarballURL,
	}
}

// ToVerification convert a git.Commit.Signature to an api.PayloadCommitVerification
func ToVerification(ctx context.Context, c *git.Commit) *api.PayloadCommitVerification {
	verif := asymkey_service.ParseCommitWithSignature(ctx, c)
	commitVerification := &api.PayloadCommitVerification{
		Verified: verif.Verified,
		Reason:   verif.Reason,
	}
	if c.Signature != nil {
		commitVerification.Signature = c.Signature.Signature
		commitVerification.Payload = c.Signature.Payload
	}
	if verif.SigningUser != nil {
		commitVerification.Signer = &api.PayloadUser{
			UserName: verif.SigningUser.Name,
			Name:     verif.SigningUser.DisplayName(),
			Email:    verif.SigningEmail, // Use the email from the signature, not from the user profile
		}
	}
	return commitVerification
}

// ToPublicKey convert asymkey_model.PublicKey to api.PublicKey
func ToPublicKey(apiLink string, key *asymkey_model.PublicKey) *api.PublicKey {
	return &api.PublicKey{
		ID:          key.ID,
		Key:         key.Content,
		URL:         fmt.Sprintf("%s%d", apiLink, key.ID),
		Title:       key.Name,
		Fingerprint: key.Fingerprint,
		Created:     key.CreatedUnix.AsTime(),
		Updated:     key.UpdatedUnix.AsTime(),
	}
}

// ToGPGKey converts models.GPGKey to api.GPGKey
func ToGPGKey(key *asymkey_model.GPGKey) *api.GPGKey {
	subkeys := make([]*api.GPGKey, len(key.SubsKey))
	for id, k := range key.SubsKey {
		subkeys[id] = &api.GPGKey{
			ID:                k.ID,
			PrimaryKeyID:      k.PrimaryKeyID,
			KeyID:             k.KeyID,
			PublicKey:         k.Content,
			Created:           k.CreatedUnix.AsTime(),
			Expires:           k.ExpiredUnix.AsTime(),
			CanSign:           k.CanSign,
			CanEncryptComms:   k.CanEncryptComms,
			CanEncryptStorage: k.CanEncryptStorage,
			CanCertify:        k.CanSign,
			Verified:          k.Verified,
		}
	}
	emails := make([]*api.GPGKeyEmail, len(key.Emails))
	for i, e := range key.Emails {
		emails[i] = ToGPGKeyEmail(e)
	}
	return &api.GPGKey{
		ID:                key.ID,
		PrimaryKeyID:      key.PrimaryKeyID,
		KeyID:             key.KeyID,
		PublicKey:         key.Content,
		Created:           key.CreatedUnix.AsTime(),
		Expires:           key.ExpiredUnix.AsTime(),
		Emails:            emails,
		SubsKey:           subkeys,
		CanSign:           key.CanSign,
		CanEncryptComms:   key.CanEncryptComms,
		CanEncryptStorage: key.CanEncryptStorage,
		CanCertify:        key.CanSign,
		Verified:          key.Verified,
	}
}

// ToGPGKeyEmail convert models.EmailAddress to api.GPGKeyEmail
func ToGPGKeyEmail(email *user_model.EmailAddress) *api.GPGKeyEmail {
	return &api.GPGKeyEmail{
		Email:    email.Email,
		Verified: email.IsActivated,
	}
}

// ToGitHook convert git.Hook to api.GitHook
func ToGitHook(h *git.Hook) *api.GitHook {
	return &api.GitHook{
		Name:     h.Name(),
		IsActive: h.IsActive,
		Content:  h.Content,
	}
}

// ToDeployKey convert deploykey_model.DeployKey to api.DeployKey
func ToDeployKey(ctx context.Context, repo *repo_model.Repository, deployKey *deploykey_model.DeployKey) *api.DeployKey {
	k := &api.DeployKey{
		ID:          deployKey.ID,
		KeyType:     util.Iif(deployKey.KeyType == deploykey_model.KeyTypeSSH, "ssh", "token"),
		KeyID:       deployKey.KeyID,
		Token:       deployKey.Token,
		URL:         repo.APIURL(ctx) + fmt.Sprintf("/keys/%d", deployKey.ID),
		Title:       deployKey.Name,
		Fingerprint: deployKey.Fingerprint,
		Created:     deployKey.CreatedUnix.AsTime(),
		ReadOnly:    deployKey.IsReadOnly(),
	}
	if deployKey.KeyType == deploykey_model.KeyTypeSSH && deployKey.LoadPublicKey(ctx) == nil {
		k.Key = deployKey.PublicKey.Content
	}
	return k
}

// ToOrganization convert user_model.User to api.Organization
func ToOrganization(ctx context.Context, org *organization.Organization) *api.Organization {
	return &api.Organization{
		ID:                        org.ID,
		AvatarURL:                 org.AsUser().AvatarLink(ctx),
		Name:                      org.Name,
		UserName:                  org.Name,
		FullName:                  org.FullName,
		Email:                     org.Email,
		Description:               org.Description,
		Website:                   org.Website,
		Location:                  org.Location,
		Visibility:                api.VisibilityString(org.Visibility.String()),
		RepoAdminChangeTeamAccess: org.RepoAdminChangeTeamAccess,
	}
}

// ToTeam convert models.Team to api.Team
func ToTeam(ctx context.Context, team *organization.Team, loadOrg ...bool) (*api.Team, error) {
	teams, err := ToTeams(ctx, []*organization.Team{team}, len(loadOrg) != 0 && loadOrg[0])
	if err != nil || len(teams) == 0 {
		return nil, err
	}
	return teams[0], nil
}

// ToTeams convert models.Team list to api.Team list
func ToTeams(ctx context.Context, teams []*organization.Team, loadOrgs bool) ([]*api.Team, error) {
	cache := make(map[int64]*api.Organization)
	apiTeams := make([]*api.Team, 0, len(teams))
	for _, t := range teams {
		if err := t.LoadUnits(ctx); err != nil {
			return nil, err
		}

		unitsMap := t.GetUnitsMap()
		apiTeam := &api.Team{
			ID:                      t.ID,
			Name:                    t.Name,
			Description:             t.Description,
			IncludesAllRepositories: t.IncludesAllRepositories,
			CanCreateOrgRepo:        t.CanCreateOrgRepo,
			Permission:              api.AccessLevelName(t.AccessMode.ToString()),
			Units:                   slices.Collect(maps.Keys(unitsMap)),
			UnitsMap:                t.GetUnitsMap(),
			Visibility:              api.TeamVisibility(t.Visibility.String()),
		}

		if loadOrgs {
			apiOrg, ok := cache[t.OrgID]
			if !ok {
				org, err := organization.GetOrgByID(ctx, t.OrgID)
				if err != nil {
					return nil, err
				}
				apiOrg = ToOrganization(ctx, org)
				cache[t.OrgID] = apiOrg
			}
			apiTeam.Organization = apiOrg
		}

		apiTeams = append(apiTeams, apiTeam)
	}
	return apiTeams, nil
}

// ToAnnotatedTag convert git.Tag to api.AnnotatedTag
func ToAnnotatedTag(ctx context.Context, repo *repo_model.Repository, t *git.Tag, c *git.Commit) *api.AnnotatedTag {
	return &api.AnnotatedTag{
		Tag:          t.Name,
		SHA:          t.ID.String(),
		Object:       ToAnnotatedTagObject(repo, c),
		Message:      t.MessageUTF8(),
		URL:          repo.APIURL() + "/git/tags/" + t.ID.String(),
		Tagger:       ToCommitUser(t.Tagger),
		Verification: ToVerification(ctx, c),
	}
}

// ToAnnotatedTagObject convert a git.Commit to an api.AnnotatedTagObject
func ToAnnotatedTagObject(repo *repo_model.Repository, commit *git.Commit) *api.AnnotatedTagObject {
	return &api.AnnotatedTagObject{
		SHA:  commit.ID.String(),
		Type: string(git.ObjectCommit),
		URL:  repo.APIURL() + "/git/commits/" + commit.ID.String(),
	}
}

// ToTagProtection convert a git.ProtectedTag to an api.TagProtection
func ToTagProtection(ctx context.Context, pt *git_model.ProtectedTag, repo *repo_model.Repository) *api.TagProtection {
	readers, err := access_model.GetUsersWithAnyUnitAccess(ctx, repo, perm.AccessModeRead, unit.TypeCode, unit.TypePullRequests)
	if err != nil {
		log.Error("GetRepoReaders: %v", err)
	}

	whitelistUsernames := getWhitelistEntities(readers, pt.AllowlistUserIDs)

	teamReaders, err := organization.GetTeamsWithAccessToAnyRepoUnit(ctx, repo.Owner.ID, repo.ID, perm.AccessModeRead, unit.TypeCode, unit.TypePullRequests)
	if err != nil {
		log.Error("Repo.Owner.TeamsWithAccessToRepo: %v", err)
	}

	whitelistTeams := getWhitelistEntities(teamReaders, pt.AllowlistTeamIDs)

	return &api.TagProtection{
		ID:                 pt.ID,
		NamePattern:        pt.NamePattern,
		WhitelistUsernames: whitelistUsernames,
		WhitelistTeams:     whitelistTeams,
		Created:            pt.CreatedUnix.AsTime(),
		Updated:            pt.UpdatedUnix.AsTime(),
	}
}

// ToTopicResponse convert from models.Topic to api.TopicResponse
func ToTopicResponse(topic *repo_model.Topic) *api.TopicResponse {
	return &api.TopicResponse{
		ID:        topic.ID,
		Name:      topic.Name,
		RepoCount: topic.RepoCount,
		Created:   topic.CreatedUnix.AsTime(),
		Updated:   topic.UpdatedUnix.AsTime(),
	}
}

// ToOAuth2Application convert from auth.OAuth2Application to api.OAuth2Application
func ToOAuth2Application(app *auth.OAuth2Application) *api.OAuth2Application {
	return &api.OAuth2Application{
		ID:                         app.ID,
		Name:                       app.Name,
		ClientID:                   app.ClientID,
		ClientSecret:               app.ClientSecret,
		ConfidentialClient:         app.ConfidentialClient,
		SkipSecondaryAuthorization: app.SkipSecondaryAuthorization,
		RedirectURIs:               app.RedirectURIs,
		Created:                    app.CreatedUnix.AsTime(),
	}
}

// ToLFSLock convert a LFSLock to api.LFSLock
func ToLFSLock(ctx context.Context, l *git_model.LFSLock) *api.LFSLock {
	_, u, err := user_model.GetPossibleUserByID(ctx, l.OwnerID)
	if err != nil {
		return nil
	}
	return &api.LFSLock{
		ID:       strconv.FormatInt(l.ID, 10),
		Path:     l.Path,
		LockedAt: l.Created.Round(time.Second),
		Owner: &api.LFSLockOwner{
			Name: u.Name,
		},
	}
}

// ToChangedFile convert a gitdiff.DiffFile to api.ChangedFile
func ToChangedFile(f *gitdiff.DiffFile, repo *repo_model.Repository, commit string) *api.ChangedFile {
	status := "changed"
	previousFilename := ""
	if f.IsDeleted {
		status = "deleted"
	} else if f.IsCreated {
		status = "added"
	} else if f.IsRenamed && f.Type == gitdiff.DiffFileCopy {
		status = "copied"
	} else if f.IsRenamed && f.Type == gitdiff.DiffFileRename {
		status = "renamed"
		previousFilename = f.OldName
	} else if f.Addition == 0 && f.Deletion == 0 {
		status = "unchanged"
	}

	file := &api.ChangedFile{
		Filename:         f.GetDiffFileName(),
		Status:           status,
		Additions:        f.Addition,
		Deletions:        f.Deletion,
		Changes:          f.Addition + f.Deletion,
		PreviousFilename: previousFilename,
		HTMLURL:          fmt.Sprint(repo.HTMLURL(), "/src/commit/", commit, "/", util.PathEscapeSegments(f.GetDiffFileName())),
		ContentsURL:      fmt.Sprint(repo.APIURL(), "/contents/", util.PathEscapeSegments(f.GetDiffFileName()), "?ref=", commit),
		RawURL:           fmt.Sprint(repo.HTMLURL(), "/raw/commit/", commit, "/", util.PathEscapeSegments(f.GetDiffFileName())),
	}

	return file
}
