// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"gitea.com/macaron/binding"
	"gitea.com/macaron/macaron"
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
	AttributesInBind              bool
	UsePagedSearch                bool
	SearchPageSize                int
	Filter                        string
	AdminFilter                   string
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
	PAMServiceName                string
	Oauth2Provider                string
	Oauth2Key                     string
	Oauth2Secret                  string
	OpenIDConnectAutoDiscoveryURL string
	Oauth2UseCustomURL            bool
	Oauth2TokenURL                string
	Oauth2AuthURL                 string
	Oauth2ProfileURL              string
	Oauth2EmailURL                string
	SSPIAutoCreateUsers           bool
	SSPIAutoActivateUsers         bool
	SSPIStripDomainNames          bool
	SSPISeparatorReplacement      string `binding:"AlphaDashDot;MaxSize(5)"`
	SSPIDefaultLanguage           string
}

// Validate validates fields
func (f *AuthenticationForm) Validate(ctx *macaron.Context, errs binding.Errors) binding.Errors {
	return validate(errs, ctx.Data, f, ctx.Locale)
}
