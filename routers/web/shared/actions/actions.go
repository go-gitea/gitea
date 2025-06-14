// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"
)

// AddSettingsRunnersRoutes adds routes for settings runners
func AddSettingsRunnersRoutes(m *web.Router) {
	m.Group("/runners", func() {
		m.Get("", Runners)
		m.Combo("/{runnerid}").Get(RunnersEdit).
			Post(web.Bind(forms.EditRunnerForm{}), RunnersEditPost)
		m.Post("/{runnerid}/delete", RunnerDeletePost)
		m.Post("/reset_registration_token", ResetRunnerRegistrationToken)
	})
}

func AddSettingsVariablesRoutes(m *web.Router) {
	m.Group("/variables", func() {
		m.Get("", Variables)
		m.Post("/new", web.Bind(forms.EditVariableForm{}), VariableCreate)
		m.Post("/{variable_id}/edit", web.Bind(forms.EditVariableForm{}), VariableUpdate)
		m.Post("/{variable_id}/delete", VariableDelete)
	})
}
