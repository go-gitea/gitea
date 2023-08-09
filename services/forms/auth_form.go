// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package forms

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/web/middleware"

	"gitea.com/go-chi/binding"
)

// AuthenticationForm form for authentication
type AuthenticationForm struct {
	ID                            int64
	Type                          int    `binding:"Range(2,7)"`
	Name                          string `binding:"Required;MaxSize(30)"`
	Host                          string
	Port                          int
	BindDN                        string
	BindPassword                  string
	UserBase                      string
	UserDN                        string
	AttributeUsername             string
	AttributeName                 string
	AttributeSurname              string
	AttributeMail                 string
	AttributeSSHPublicKey         string
	AttributeAvatar               string
	AttributesInBind              bool
	UsePagedSearch                bool
	SearchPageSize                int
	Filter                        string
	AdminFilter                   string
	GroupsEnabled                 bool
	GroupDN                       string
	GroupFilter                   string
	GroupMemberUID                string
	UserUID                       string
	RestrictedFilter              string
	AllowDeactivateAll            bool
	IsActive                      bool
	IsSyncEnabled                 bool
	SMTPAuth                      string
	SMTPHost                      string
	SMTPPort                      int
	AllowedDomains                string
	SecurityProtocol              int `binding:"Range(0,2)"`
	TLS                           bool
	SkipVerify                    bool
	HeloHostname                  string
	DisableHelo                   bool
	ForceSMTPS                    bool
	PAMServiceName                string
	PAMEmailDomain                string
	Oauth2Provider                string
	Oauth2Key                     string
	Oauth2Secret                  string
	OpenIDConnectAutoDiscoveryURL string
	Oauth2UseCustomURL            bool
	Oauth2TokenURL                string
	Oauth2AuthURL                 string
	Oauth2ProfileURL              string
	Oauth2EmailURL                string
	Oauth2IconURL                 string
	Oauth2Tenant                  string
	Oauth2Scopes                  string
	Oauth2RequiredClaimName       string
	Oauth2RequiredClaimValue      string
	Oauth2GroupClaimName          string
	Oauth2AdminGroup              string
	Oauth2RestrictedGroup         string
	Oauth2GroupTeamMap            string `binding:"ValidGroupTeamMap"`
	Oauth2GroupTeamMapRemoval     bool
	SkipLocalTwoFA                bool
	SSPIAutoCreateUsers           bool
	SSPIAutoActivateUsers         bool
	SSPIStripDomainNames          bool
	SSPISeparatorReplacement      string `binding:"AlphaDashDot;MaxSize(5)"`
	SSPIDefaultLanguage           string
	GroupTeamMap                  string `binding:"ValidGroupTeamMap"`
	GroupTeamMapRemoval           bool
}

// Validate validates fields
func (f *AuthenticationForm) Validate(req *http.Request, errs binding.Errors) binding.Errors {
	ctx := context.GetValidateContext(req)
	return middleware.Validate(errs, ctx.Data, f, ctx.Locale)
}
