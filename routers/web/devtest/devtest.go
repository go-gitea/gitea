// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package devtest

import (
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"

	"code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/badge"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/templates"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/context"
)

// List all devtest templates, they will be used for e2e tests for the UI components
func List(ctx *context.Context) {
	templateNames, err := templates.AssetFS().ListFiles("devtest", true)
	if err != nil {
		ctx.ServerError("AssetFS().ListFiles", err)
		return
	}
	var subNames []string
	for _, tmplName := range templateNames {
		subName := strings.TrimSuffix(tmplName, ".tmpl")
		if !strings.HasPrefix(subName, "devtest-") {
			subNames = append(subNames, subName)
		}
	}
	ctx.Data["SubNames"] = subNames
	ctx.HTML(http.StatusOK, "devtest/devtest-list")
}

func FetchActionTest(ctx *context.Context) {
	_ = ctx.Req.ParseForm()
	ctx.Flash.Info("fetch-action: " + ctx.Req.Method + " " + ctx.Req.RequestURI + "<br>" +
		"Form: " + ctx.Req.Form.Encode() + "<br>" +
		"PostForm: " + ctx.Req.PostForm.Encode(),
	)
	time.Sleep(2 * time.Second)
	ctx.JSONRedirect("")
}

func prepareMockDataGiteaUI(ctx *context.Context) {
	now := time.Now()
	ctx.Data["TimeNow"] = now
	ctx.Data["TimePast5s"] = now.Add(-5 * time.Second)
	ctx.Data["TimeFuture5s"] = now.Add(5 * time.Second)
	ctx.Data["TimePast2m"] = now.Add(-2 * time.Minute)
	ctx.Data["TimeFuture2m"] = now.Add(2 * time.Minute)
	ctx.Data["TimePast1y"] = now.Add(-1 * 366 * 86400 * time.Second)
	ctx.Data["TimeFuture1y"] = now.Add(1 * 366 * 86400 * time.Second)
}

func prepareMockDataBadgeCommitSign(ctx *context.Context) {
	var commits []*asymkey.SignCommit
	mockUsers, _ := db.Find[user_model.User](ctx, user_model.SearchUserOptions{ListOptions: db.ListOptions{PageSize: 1}})
	mockUser := mockUsers[0]
	commits = append(commits, &asymkey.SignCommit{
		Verification: &asymkey.CommitVerification{},
		UserCommit: &user_model.UserCommit{
			Commit: &git.Commit{ID: git.Sha1ObjectFormat.EmptyObjectID()},
		},
	})
	commits = append(commits, &asymkey.SignCommit{
		Verification: &asymkey.CommitVerification{
			Verified:    true,
			Reason:      "name / key-id",
			SigningUser: mockUser,
			SigningKey:  &asymkey.GPGKey{KeyID: "12345678"},
			TrustStatus: "trusted",
		},
		UserCommit: &user_model.UserCommit{
			User:   mockUser,
			Commit: &git.Commit{ID: git.Sha1ObjectFormat.EmptyObjectID()},
		},
	})
	commits = append(commits, &asymkey.SignCommit{
		Verification: &asymkey.CommitVerification{
			Verified:      true,
			Reason:        "name / key-id",
			SigningUser:   mockUser,
			SigningSSHKey: &asymkey.PublicKey{Fingerprint: "aa:bb:cc:dd:ee"},
			TrustStatus:   "untrusted",
		},
		UserCommit: &user_model.UserCommit{
			User:   mockUser,
			Commit: &git.Commit{ID: git.Sha1ObjectFormat.EmptyObjectID()},
		},
	})
	commits = append(commits, &asymkey.SignCommit{
		Verification: &asymkey.CommitVerification{
			Verified:      true,
			Reason:        "name / key-id",
			SigningUser:   mockUser,
			SigningSSHKey: &asymkey.PublicKey{Fingerprint: "aa:bb:cc:dd:ee"},
			TrustStatus:   "other(unmatch)",
		},
		UserCommit: &user_model.UserCommit{
			User:   mockUser,
			Commit: &git.Commit{ID: git.Sha1ObjectFormat.EmptyObjectID()},
		},
	})
	commits = append(commits, &asymkey.SignCommit{
		Verification: &asymkey.CommitVerification{
			Warning:      true,
			Reason:       "gpg.error",
			SigningEmail: "test@example.com",
		},
		UserCommit: &user_model.UserCommit{
			User:   mockUser,
			Commit: &git.Commit{ID: git.Sha1ObjectFormat.EmptyObjectID()},
		},
	})

	ctx.Data["MockCommits"] = commits
}

func prepareMockDataBadgeActionsSvg(ctx *context.Context) {
	fontFamilyNames := strings.Split(badge.DefaultFontFamily, ",")
	selectedFontFamilyName := ctx.FormString("font", fontFamilyNames[0])
	selectedStyle := ctx.FormString("style", badge.DefaultStyle)
	var badges []badge.Badge
	badges = append(badges, badge.GenerateBadge("啊啊啊啊啊啊啊啊啊啊啊啊", "🌞🌞🌞🌞🌞", "green"))
	for r := rune(0); r < 256; r++ {
		if unicode.IsPrint(r) {
			s := strings.Repeat(string(r), 15)
			badges = append(badges, badge.GenerateBadge(s, util.TruncateRunes(s, 7), "green"))
		}
	}

	var badgeSVGs []template.HTML
	for i, b := range badges {
		b.IDPrefix = "devtest-" + strconv.FormatInt(int64(i), 10) + "-"
		b.FontFamily = selectedFontFamilyName
		var h template.HTML
		var err error
		switch selectedStyle {
		case badge.StyleFlat:
			h, err = ctx.RenderToHTML("shared/actions/runner_badge_flat", map[string]any{"Badge": b})
		case badge.StyleFlatSquare:
			h, err = ctx.RenderToHTML("shared/actions/runner_badge_flat-square", map[string]any{"Badge": b})
		default:
			err = fmt.Errorf("unknown badge style: %s", selectedStyle)
		}
		if err != nil {
			ctx.ServerError("RenderToHTML", err)
			return
		}
		badgeSVGs = append(badgeSVGs, h)
	}
	ctx.Data["BadgeSVGs"] = badgeSVGs
	ctx.Data["BadgeFontFamilyNames"] = fontFamilyNames
	ctx.Data["SelectedFontFamilyName"] = selectedFontFamilyName
	ctx.Data["BadgeStyles"] = badge.GlobalVars().AllStyles
	ctx.Data["SelectedStyle"] = selectedStyle
}

func prepareMockData(ctx *context.Context) {
	switch ctx.Req.URL.Path {
	case "/devtest/gitea-ui":
		prepareMockDataGiteaUI(ctx)
	case "/devtest/badge-commit-sign":
		prepareMockDataBadgeCommitSign(ctx)
	case "/devtest/badge-actions-svg":
		prepareMockDataBadgeActionsSvg(ctx)
	}
}

func TmplCommon(ctx *context.Context) {
	prepareMockData(ctx)
	if ctx.Req.Method == http.MethodPost {
		_ = ctx.Req.ParseForm()
		ctx.Flash.Info("form: "+ctx.Req.Method+" "+ctx.Req.RequestURI+"<br>"+
			"Form: "+ctx.Req.Form.Encode()+"<br>"+
			"PostForm: "+ctx.Req.PostForm.Encode(),
			true,
		)
		time.Sleep(2 * time.Second)
	}
	ctx.HTML(http.StatusOK, templates.TplName("devtest"+path.Clean("/"+ctx.PathParam("sub"))))
}
