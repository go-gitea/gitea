// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package agent

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"strings"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/optional"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/modules/web"
	agent_service "code.gitea.io/gitea/services/agent"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/convert"
	repo_service "code.gitea.io/gitea/services/repository"
)

const (
	internalTokenHeader = "X-Internal-Token"
	defaultTokenName    = "agent-enroll-token"
)

// Enroll creates a new agent account and a bootstrap access token.
func Enroll(ctx *context.APIContext) {
	// swagger:operation POST /agents/enroll agent agentEnroll
	// ---
	// summary: Enroll a new agent account
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: X-Internal-Token
	//   in: header
	//   type: string
	//   required: false
	// - name: body
	//   in: body
	//   schema:
	//     "$ref": "#/definitions/AgentEnrollOption"
	// responses:
	//   "201":
	//     "$ref": "#/responses/AgentEnrollResponse"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "422":
	//     "$ref": "#/responses/validationError"

	token := ctx.Req.Header.Get(internalTokenHeader)
	if !agent_service.IsEnrollmentEnabled(ctx.Req.Context()) {
		ctx.APIError(http.StatusForbidden, "agent enrollment is disabled")
		return
	}
	if !agent_service.IsEnrollmentRequestAllowed(ctx.Req.Context(), ctx.Req.RemoteAddr) {
		ctx.APIError(http.StatusForbidden, "enrollment source address is not allowed")
		return
	}
	if setting.Config().Agent.RequireInternalToken.Value(ctx.Req.Context()) {
		if setting.InternalToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(setting.InternalToken)) != 1 {
			ctx.APIError(http.StatusForbidden, "invalid internal enrollment token")
			return
		}
	}

	form := web.GetForm(ctx).(*api.AgentEnrollOption)
	normalizedUsername, err := agent_service.NormalizeEnrollmentUsername(form.Username)
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err)
		return
	}

	passwd, err := util.CryptoRandomString(40)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	email := strings.TrimSpace(form.Email)
	networkIdentifier := strings.TrimSpace(form.NetworkIdentifier)
	if email != "" {
		if ip := net.ParseIP(email); ip != nil {
			if networkIdentifier == "" {
				networkIdentifier = ip.String()
			}
			email = ""
		} else if _, err := mail.ParseAddress(email); err != nil {
			ctx.APIError(http.StatusUnprocessableEntity, errors.New("email must be a valid email address, IPs belong in network_identifier"))
			return
		}
	}
	if email == "" {
		suffix, err := util.CryptoRandomString(8)
		if err != nil {
			ctx.APIErrorInternal(err)
			return
		}
		email = fmt.Sprintf("%s+%s@%s", normalizedUsername, strings.ToLower(suffix), setting.Service.NoReplyAddress)
	}
	u := &user_model.User{
		Name:          normalizedUsername,
		FullName:      form.FullName,
		Email:         email,
		Passwd:        passwd,
		Type:          user_model.UserTypeBot,
		ProhibitLogin: false,
		LoginType:     auth.Plain,
	}
	overwrites := &user_model.CreateUserOverwriteOptions{
		IsActive:                optional.Some(true),
		IsRestricted:            optional.Some(true),
		AllowCreateOrganization: optional.Some(false),
	}
	if err := user_model.AdminCreateUser(ctx, u, &user_model.Meta{}, overwrites); err != nil {
		errMsg := strings.ToLower(err.Error())
		if user_model.IsErrUserAlreadyExist(err) || strings.Contains(errMsg, "user already exists") {
			existing := &user_model.User{}
			has, getErr := db.GetEngine(ctx).Where("lower_name = ? OR name = ?", strings.ToLower(normalizedUsername), normalizedUsername).Get(existing)
			if getErr != nil {
				ctx.APIErrorInternal(getErr)
				return
			}
			if !has {
				ctx.APIError(http.StatusUnprocessableEntity, err)
				return
			}
			// Re-enrollment is only supported for existing bot/agent accounts.
			if existing.Type != user_model.UserTypeBot {
				ctx.APIError(http.StatusUnprocessableEntity, errors.New("existing user is not an agent account"))
				return
			}
			existing.IsActive = true
			existing.IsRestricted = true
			existing.ProhibitLogin = false
			if strings.TrimSpace(form.FullName) != "" {
				existing.FullName = form.FullName
			}
			if updateErr := user_model.UpdateUserCols(ctx, existing, "is_active", "is_restricted", "prohibit_login", "full_name"); updateErr != nil {
				ctx.APIErrorInternal(updateErr)
				return
			}
			u = existing
		} else {
			switch {
			case user_model.IsErrEmailAlreadyUsed(err),
				db.IsErrNameReserved(err),
				db.IsErrNameCharsNotAllowed(err),
				user_model.IsErrEmailCharIsNotSupported(err),
				user_model.IsErrEmailInvalid(err),
				db.IsErrNamePatternNotAllowed(err):
				ctx.APIError(http.StatusUnprocessableEntity, err)
			default:
				ctx.APIErrorInternal(err)
			}
			return
		}
	}
	if u.ID == 0 {
		ctx.APIErrorInternal(errors.New("invalid user during agent enrollment"))
		return
	}
	if u.Name == "" {
		ctx.APIErrorInternal(errors.New("empty user during agent enrollment"))
		return
	}
	u.IsActive = true
	u.ProhibitLogin = false
	if err := user_model.UpdateUserCols(ctx, u, "is_active", "prohibit_login"); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	u, err = user_model.GetUserByID(ctx, u.ID)
	if err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ownerAgentValue := "false"
	if form.OwnerAgent {
		ownerAgentValue = "true"
	}
	if err := user_model.SetUserSetting(ctx, u.ID, agent_service.SettingOwnerAgent, ownerAgentValue); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	if err := user_model.SetUserSetting(ctx, u.ID, agent_service.SettingRequestedUsername, strings.TrimSpace(form.Username)); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	machineIdentity := strings.TrimSpace(form.MachineIdentity)
	if machineIdentity == "" {
		machineIdentity = strings.TrimSpace(form.Username)
	}
	if machineIdentity != "" {
		if err := user_model.SetUserSetting(ctx, u.ID, agent_service.SettingMachineIdentity, machineIdentity); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	if networkIdentifier != "" {
		if err := user_model.SetUserSetting(ctx, u.ID, agent_service.SettingNetworkIdentifier, networkIdentifier); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	if setting.Config().Agent.AutoCreateRepo.Value(ctx.Req.Context()) {
		repoName := strings.TrimSpace(setting.Config().Agent.AutoCreateRepoName.Value(ctx.Req.Context()))
		switch strings.ToLower(repoName) {
		case "", "{username}", "$username", "username", "self":
			repoName = normalizedUsername
		}
		if repo_model.IsUsableRepoName(repoName) != nil {
			repoName = normalizedUsername
		}
		_, err := repo_service.CreateRepository(ctx, u, u, repo_service.CreateRepoOptions{
			Name:      repoName,
			IsPrivate: setting.Config().Agent.AutoCreateRepoIsPrivate.Value(ctx.Req.Context()),
		})
		if err != nil && !repo_model.IsErrRepoAlreadyExist(err) {
			ctx.APIErrorInternal(err)
			return
		}
	}

	tokenName := form.TokenName
	if tokenName == "" {
		tokenName = defaultTokenName
	}
	tokenScopes := form.TokenScopes
	if len(tokenScopes) == 0 {
		if setting.Config().Agent.AutoCreateRepo.Value(ctx.Req.Context()) {
			tokenScopes = []string{"write:repository", "write:user"}
		} else {
			tokenScopes = []string{"public-only", "read:repository"}
		}
	}
	scope, err := auth.AccessTokenScope(strings.Join(tokenScopes, ",")).Normalize()
	if err != nil {
		ctx.APIError(http.StatusUnprocessableEntity, err)
		return
	}
	if scope == "" {
		ctx.APIError(http.StatusUnprocessableEntity, errors.New("token scope must not be empty"))
		return
	}

	bootstrapToken := &auth.AccessToken{
		UID:   u.ID,
		Name:  tokenName,
		Scope: scope,
	}
	// Token rotation: revoke previous token(s) with same name for this user.
	var existingTokens []*auth.AccessToken
	if err := db.GetEngine(ctx).Where("uid = ? AND name = ?", u.ID, tokenName).Find(&existingTokens); err != nil {
		ctx.APIErrorInternal(err)
		return
	}
	for _, t := range existingTokens {
		if err := auth.DeleteAccessTokenByID(ctx, t.ID, u.ID); err != nil {
			ctx.APIErrorInternal(err)
			return
		}
	}
	if err := auth.NewAccessToken(ctx, bootstrapToken); err != nil {
		ctx.APIErrorInternal(err)
		return
	}

	ctx.JSON(http.StatusCreated, &api.AgentEnrollResponse{
		User:         convert.ToUser(ctx, u, ctx.Doer),
		Token:        bootstrapToken.Token,
		TokenName:    bootstrapToken.Name,
		TokenScopes:  bootstrapToken.Scope.StringSlice(),
		IsOwnerAgent: form.OwnerAgent,
	})
}
