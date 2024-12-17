// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	stdCtx "context"
	"fmt"
	"net/http"
	"strconv"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/private"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/mailer"
	sender_service "code.gitea.io/gitea/services/mailer/sender"
)

// SendEmail pushes messages to mail queue
//
// It doesn't wait before each message will be processed
func SendEmail(ctx *context.PrivateContext) {
	if setting.MailService == nil {
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: "Mail service is not enabled.",
		})
		return
	}

	var mail private.Email
	rd := ctx.Req.Body
	defer rd.Close()

	if err := json.NewDecoder(rd).Decode(&mail); err != nil {
		log.Error("JSON Decode failed: %v", err)
		ctx.JSON(http.StatusInternalServerError, private.Response{
			Err: err.Error(),
		})
		return
	}

	var emails []string
	if len(mail.To) > 0 {
		for _, uname := range mail.To {
			user, err := user_model.GetUserByName(ctx, uname)
			if err != nil {
				err := fmt.Sprintf("Failed to get user information: %v", err)
				log.Error(err)
				ctx.JSON(http.StatusInternalServerError, private.Response{
					Err: err,
				})
				return
			}

			if user != nil && len(user.Email) > 0 {
				emails = append(emails, user.Email)
			}
		}
	} else {
		err := db.Iterate(ctx, nil, func(ctx stdCtx.Context, user *user_model.User) error {
			if len(user.Email) > 0 && user.IsActive {
				emails = append(emails, user.Email)
			}
			return nil
		})
		if err != nil {
			err := fmt.Sprintf("Failed to find users: %v", err)
			log.Error(err)
			ctx.JSON(http.StatusInternalServerError, private.Response{
				Err: err,
			})
			return
		}
	}

	sendEmail(ctx, mail.Subject, mail.Message, emails)
}

func sendEmail(ctx *context.PrivateContext, subject, message string, to []string) {
	for _, email := range to {
		msg := sender_service.NewMessage(email, subject, message)
		mailer.SendAsync(msg)
	}

	wasSent := strconv.Itoa(len(to))

	ctx.PlainText(http.StatusOK, wasSent)
}
