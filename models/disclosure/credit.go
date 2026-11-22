// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package disclosure

import (
	"net/url"
	"strings"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/json"
	"gitea.dev/modules/util"
)

// CreditRole is the role a credited person played in the disclosure (CVE CNA vocabulary)
type CreditRole string

// Credit roles, following the CVE CNA credit vocabulary
const (
	CreditRoleFinder      CreditRole = "finder"
	CreditRoleReporter    CreditRole = "reporter"
	CreditRoleAnalyst     CreditRole = "analyst"
	CreditRoleCoordinator CreditRole = "coordinator"
	CreditRoleSponsor     CreditRole = "sponsor"
	CreditRoleOther       CreditRole = "other"
)

// AllCreditRoles lists the roles in dropdown order
var AllCreditRoles = []CreditRole{
	CreditRoleFinder,
	CreditRoleReporter,
	CreditRoleAnalyst,
	CreditRoleCoordinator,
	CreditRoleSponsor,
	CreditRoleOther,
}

// ParseCreditRole parses a credit role string, returning an error for unknown values
func ParseCreditRole(s string) (CreditRole, error) {
	switch CreditRole(s) {
	case CreditRoleFinder, CreditRoleReporter, CreditRoleAnalyst, CreditRoleCoordinator, CreditRoleSponsor, CreditRoleOther:
		return CreditRole(s), nil
	}
	return "", util.NewInvalidArgumentErrorf("invalid credit role %q", s)
}

const (
	// CreditNameMaxLength is the maximum length of a credit name
	CreditNameMaxLength = 255
	// CreditURLMaxLength is the maximum length of an external credit's URL
	CreditURLMaxLength = 2048
	// CreditsMaxCount is the maximum number of credits per disclosure
	CreditsMaxCount = 32
)

// Credit is one entry in a disclosure's credit list. The "who" is either a local
// Gitea user (UserID > 0, User resolved on load) or an external person (Name + URL).
type Credit struct {
	Role   CreditRole `json:"role"`
	UserID int64      `json:"user_id,omitempty"` // 0 for external people
	Name   string     `json:"name"`              // display name; always set, also as fallback for deleted local users
	URL    string     `json:"url,omitempty"`     // optional link, external people only

	User *user_model.User `json:"-" xorm:"-"` // resolved local user, set on load
}

// Validate checks a single credit. Credits for local users (UserID > 0) were resolved
// by the service and always carry a name; external people need a name and may add a URL.
func (c *Credit) Validate() error {
	if _, err := ParseCreditRole(string(c.Role)); err != nil {
		return err
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return util.NewInvalidArgumentErrorf("credit name is required")
	}
	if len(c.Name) > CreditNameMaxLength {
		return util.NewInvalidArgumentErrorf("credit name too long")
	}
	if c.UserID == 0 {
		// external person: url optional but must be a valid http(s) URL
		c.URL = strings.TrimSpace(c.URL)
		if c.URL != "" {
			if len(c.URL) > CreditURLMaxLength {
				return util.NewInvalidArgumentErrorf("credit URL too long")
			}
			u, err := url.Parse(c.URL)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return util.NewInvalidArgumentErrorf("credit URL must be a valid http(s) URL")
			}
		}
	} else {
		c.URL = "" // local users link to their profile, no custom URL
	}
	return nil
}

// CreditList is a list of credits stored as JSON in a TEXT column
type CreditList []*Credit

// FromDB implements the convert.Conversion interface
func (l *CreditList) FromDB(bs []byte) error {
	if len(bs) == 0 {
		*l = nil
		return nil
	}
	return json.Unmarshal(bs, l)
}

// ToDB implements the convert.Conversion interface
func (l CreditList) ToDB() ([]byte, error) {
	if len(l) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(l)
}

// ValidateCredits checks a whole credit list: roles, names, URLs and the count limit
func ValidateCredits(credits CreditList) error {
	if len(credits) > CreditsMaxCount {
		return util.NewInvalidArgumentErrorf("too many credits (max %d)", CreditsMaxCount)
	}
	for _, c := range credits {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}
