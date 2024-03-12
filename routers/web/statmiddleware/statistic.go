package statmiddleware

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/messagequeue"
	"code.gitea.io/gitea/modules/setting"
)

// Constants for topics, types, and field names.
const (
	gitClone    = "git_clone"
	webDownload = "web_download"
)

type msgDetail struct {
	RepoId        string `json:"repo_id"`
	RepoName      string `json:"repo_name"`
	RepoOwner     string `json:"repo_owner"`
	DoerEmail     string `json:"doer_email"`
	RepoUserEmail string `json:"repo_user_email"`

	RequestIP    string `json:"request_IP"`
	RequestID    string `json:"request_ID"`
	RequestPath  string `json:"path"`
	RequestProto string `json:"request_proto"`
	RequestAgent string `json:"http_user_agent"`
}

type msgNormal struct {
	Type      string      `json:"type"`
	User      string      `json:"user"`
	Desc      string      `json:"desc"`
	Details   interface{} `json:"details"`
	CreatedAt int64       `json:"created_at"`
}

// getUserInfo extracts user information from the context.
func getUserInfo(ctx *context.Context) (string, string) {
	if ctx.Doer == nil {
		return "anno", "anno"
	}

	return ctx.Doer.Name, ctx.Doer.Email
}

// prepareMessage constructs the message to be sent.
func prepareMessage(ctx *context.Context, messageType string, description string) msgNormal {
	doer, email := getUserInfo(ctx)

	detail := msgDetail{
		RepoId:        strconv.FormatInt(ctx.Repo.Repository.ID, 10),
		RepoName:      ctx.Repo.Repository.Name,
		RepoOwner:     ctx.Repo.Repository.OwnerName,
		DoerEmail:     email,
		RepoUserEmail: ctx.Repo.Repository.Owner.Email,

		RequestID:    uuid.New().String(),
		RequestIP:    ctx.Req.RemoteAddr,
		RequestPath:  ctx.Repo.RepoLink,
		RequestAgent: ctx.Req.UserAgent(),
		RequestProto: ctx.Req.Proto,
	}

	return msgNormal{
		Type:      messageType,
		User:      doer,
		Desc:      description,
		Details:   detail,
		CreatedAt: time.Now().Unix(),
	}
}

// GitCloneMessageMiddleware middleware for send statistics message about git clone
func GitCloneMessageMiddleware(ctx *context.Context) {
	if setting.MQ == nil {
		log.Info("message component uninitialized, will skip publishing git clone message")
		return
	}
	msg := prepareMessage(
		ctx, gitClone,
		"this message means someone cloned the repository",
	)

	if err := messagequeue.Publish(setting.MQ.TopicName, &msg, nil); err != nil {
		log.Info("internal error of kafka sender: %s", err.Error())
	}
}

// WebDownloadMiddleware middleware for send statistics message about git clone
func WebDownloadMiddleware(ctx *context.Context) {
	if setting.MQ == nil {
		log.Info("message component uninitialized, will skip publishing web download message")
	}
	msg := prepareMessage(
		ctx, webDownload,
		"this message means someone downloaded a file from the website",
	)

	if err := messagequeue.Publish(setting.MQ.TopicName, &msg, nil); err != nil {
		log.Info("internal error of kafka sender: %s", err.Error())
	}
}
