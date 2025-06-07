// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"net/http"
	"strconv"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/routers/web/auth"
	"code.gitea.io/gitea/routers/web/repo/actions"
	repo_setting "code.gitea.io/gitea/routers/web/repo/setting"
	"code.gitea.io/gitea/routers/web/shared"
	shared_actions "code.gitea.io/gitea/routers/web/shared/actions"
	"code.gitea.io/gitea/routers/web/user/setting/security"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/forms"
)

func UpdatePreferences(ctx *context.Context) {
	type preferencesForm struct {
		CodeViewShowFileTree bool `json:"codeViewShowFileTree"`
	}
	form := &preferencesForm{}
	if err := json.NewDecoder(ctx.Req.Body).Decode(&form); err != nil {
		ctx.HTTPError(http.StatusBadRequest, "json decode failed")
		return
	}
	_ = user_model.SetUserSetting(ctx, ctx.Doer.ID, user_model.SettingsKeyCodeViewShowFileTree, strconv.FormatBool(form.CodeViewShowFileTree))
	ctx.JSONOK()
}

// /user/settings/* routes
func ProvideUserSettingsRoutes(m *web.Router) func() {
	return func() {
		m.Get("", Profile)
		m.Post("", web.Bind(forms.UpdateProfileForm{}), ProfilePost)
		m.Post("/update_preferences", UpdatePreferences)
		m.Get("/change_password", auth.MustChangePassword)
		m.Post("/change_password", web.Bind(forms.MustChangePasswordForm{}), auth.MustChangePasswordPost)
		m.Post("/avatar", web.Bind(forms.AvatarForm{}), AvatarPost)
		m.Post("/avatar/delete", DeleteAvatar)
		m.Group("/account", func() {
			m.Combo("").Get(Account).Post(web.Bind(forms.ChangePasswordForm{}), AccountPost)
			m.Post("/email", web.Bind(forms.AddEmailForm{}), EmailPost)
			m.Post("/email/delete", DeleteEmail)
			m.Post("/delete", DeleteAccount)
		})
		m.Group("/appearance", func() {
			m.Get("", Appearance)
			m.Post("/language", web.Bind(forms.UpdateLanguageForm{}), UpdateUserLang)
			m.Post("/hidden_comments", UpdateUserHiddenComments)
			m.Post("/theme", web.Bind(forms.UpdateThemeForm{}), UpdateUIThemePost)
		})
		m.Group("/security", func() {
			m.Get("", security.Security)
			m.Group("/two_factor", func() {
				m.Post("/regenerate_scratch", security.RegenerateScratchTwoFactor)
				m.Post("/disable", security.DisableTwoFactor)
				m.Get("/enroll", security.EnrollTwoFactor)
				m.Post("/enroll", web.Bind(forms.TwoFactorAuthForm{}), security.EnrollTwoFactorPost)
			})
			m.Group("/webauthn", func() {
				m.Post("/request_register", web.Bind(forms.WebauthnRegistrationForm{}), security.WebAuthnRegister)
				m.Post("/register", security.WebauthnRegisterPost)
				m.Post("/delete", web.Bind(forms.WebauthnDeleteForm{}), security.WebauthnDelete)
			})
			m.Group("/openid", func() {
				m.Post("", web.Bind(forms.AddOpenIDForm{}), security.OpenIDPost)
				m.Post("/delete", security.DeleteOpenID)
				m.Post("/toggle_visibility", security.ToggleOpenIDVisibility)
			}, shared.OpenIDSignInEnabled)
			m.Post("/account_link", shared.LinkAccountEnabled, security.DeleteAccountLink)
		})

		m.Group("/applications", func() {
			// oauth2 applications
			m.Group("/oauth2", func() {
				m.Get("/{id}", OAuth2ApplicationShow)
				m.Post("/{id}", web.Bind(forms.EditOAuth2ApplicationForm{}), OAuthApplicationsEdit)
				m.Post("/{id}/regenerate_secret", OAuthApplicationsRegenerateSecret)
				m.Post("", web.Bind(forms.EditOAuth2ApplicationForm{}), OAuthApplicationsPost)
				m.Post("/{id}/delete", DeleteOAuth2Application)
				m.Post("/{id}/revoke/{grantId}", RevokeOAuth2Grant)
			}, shared.Oauth2Enabled)

			// access token applications
			m.Combo("").Get(Applications).
				Post(web.Bind(forms.NewAccessTokenForm{}), ApplicationsPost)
			m.Post("/delete", DeleteApplication)
		})

		m.Combo("/keys").Get(Keys).
			Post(web.Bind(forms.AddKeyForm{}), KeysPost)
		m.Post("/keys/delete", DeleteKey)
		m.Group("/packages", func() {
			m.Get("", Packages)
			m.Group("/rules", func() {
				m.Group("/add", func() {
					m.Get("", PackagesRuleAdd)
					m.Post("", web.Bind(forms.PackageCleanupRuleForm{}), PackagesRuleAddPost)
				})
				m.Group("/{id}", func() {
					m.Get("", PackagesRuleEdit)
					m.Post("", web.Bind(forms.PackageCleanupRuleForm{}), PackagesRuleEditPost)
					m.Get("/preview", PackagesRulePreview)
				})
			})
			m.Group("/cargo", func() {
				m.Post("/initialize", InitializeCargoIndex)
				m.Post("/rebuild", RebuildCargoIndex)
			})
			m.Post("/chef/regenerate_keypair", RegenerateChefKeyPair)
		}, shared.PackagesEnabled)

		m.Group("/actions", func() {
			m.Get("", RedirectToDefaultSetting)
			shared_actions.AddSettingsRunnersRoutes(m)
			repo_setting.AddSettingsSecretsRoutes(m)
			shared_actions.AddSettingsVariablesRoutes(m)
		}, actions.MustEnableActions)

		m.Get("/organization", Organization)
		m.Get("/repos", Repos)
		m.Post("/repos/unadopted", AdoptOrDeleteRepository)

		m.Group("/hooks", func() {
			m.Get("", Webhooks)
			m.Post("/delete", DeleteWebhook)
			repo_setting.AddWebhookAddRoutes(m)
			m.Group("/{id}", func() {
				m.Get("", repo_setting.WebHooksEdit)
				m.Post("/replay/{uuid}", repo_setting.ReplayWebhook)
			})
			repo_setting.AddWebhookEditRoutes(m)
		}, shared.WebhooksEnabled)

		m.Group("/blocked_users", func() {
			m.Get("", BlockedUsers)
			m.Post("", web.Bind(forms.BlockUserForm{}), BlockedUsersPost)
		})
	}
}
