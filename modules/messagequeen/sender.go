package messagequeen

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/messagequeen/infrastructure/kafka"
	"code.gitea.io/gitea/modules/setting"
)

// Constants for topics, types, and field names.
const (
	gitClone    = "git_clone"
	webDownload = "web_download"
)

type msgDetail struct {
	RepoName      string `json:"repo_name"`
	RepoOwner     string `json:"repo_owner"`
	DoerEmail     string `json:"doer_email"`
	RepoId        string `json:"repo_id"`
	RepoUserEmail string `json:"repo_user_email"`
	RequestIP     string `json:"request_IP"`
	RequestID     string `json:"request_ID"`
	RequestProto  string `json:"request_proto"`
	RequestAgent  string `json:"http_user_agent"`
	RequestPath   string `json:"path"`
}

type MsgNormal struct {
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
func prepareMessage(ctx *context.Context, messageType string, description string) MsgNormal {
	doer, email := getUserInfo(ctx)

	// Generate a new UUID for the request.
	RequestID := uuid.New().String()

	detail := msgDetail{
		RepoName:      ctx.Repo.Repository.Name,
		RepoOwner:     ctx.Repo.Repository.OwnerName,
		DoerEmail:     email,
		RepoId:        strconv.FormatInt(ctx.Repo.Repository.ID, 10),
		RequestProto:  ctx.Req.Proto,
		RequestIP:     ctx.Req.RemoteAddr,
		RequestAgent:  ctx.Req.UserAgent(),
		RepoUserEmail: ctx.Repo.Repository.Owner.Email,
		RequestPath:   ctx.Repo.RepoLink,
		RequestID:     RequestID,
	}

	return MsgNormal{
		Type:      messageType,
		User:      doer,
		Desc:      description,
		Details:   detail,
		CreatedAt: time.Now().Unix(),
	}
}

// GitCloneSender handles sending Git clone events to Kafka.
func GitCloneSender(ctx *context.Context) error {
	msg := prepareMessage(ctx, gitClone, "this message means someone cloned the repository")
	return kafka.Publish(setting.MQ.TopicName, &msg, nil)
}

// WebDownloadSender handles sending Web download events to Kafka.
func WebDownloadSender(ctx *context.Context) error {
	msg := prepareMessage(ctx, webDownload, "this message means someone downloaded a file from the website")
	return kafka.Publish(setting.MQ.TopicName, &msg, nil)
}
