// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package types

import "code.gitea.io/gitea/modules/translation"

type OwnerType string

const (
	OwnerTypeSystemGlobal = "system-global"
	OwnerTypeIndividual   = "individual"
	OwnerTypeRepository   = "repository"
	OwnerTypeOrganization = "organization"
)

func (o OwnerType) LocaleString(locale translation.Locale) string {
	switch o {
	case OwnerTypeSystemGlobal:
		return locale.TrString("concept_system_global")
	case OwnerTypeIndividual:
		return locale.TrString("concept_user_individual")
	case OwnerTypeRepository:
		return locale.TrString("concept_code_repository")
	case OwnerTypeOrganization:
		return locale.TrString("concept_user_organization")
	}
	return locale.TrString("unknown")
}
