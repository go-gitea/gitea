package user

import (
	"strconv"
	"strings"
)

type ExtDoerData interface {
	EncodeToString() string
	DecodeFromString(string) error
}

type extDoerGiteaActions struct {
	TaskID int64
}

var _ ExtDoerData = (*extDoerGiteaActions)(nil)

func (e *extDoerGiteaActions) EncodeToString() string {
	return "gitea-actions:" + strconv.FormatInt(e.TaskID, 10)
}

func (e *extDoerGiteaActions) DecodeFromString(s string) (err error) {
	idStr, _ := strings.CutPrefix(s, "gitea-actions:")
	e.TaskID, err = strconv.ParseInt(idStr, 10, 64)
	return err
}

type extDoerDeployKey struct {
	DeployKeyID int64
}

var _ ExtDoerData = (*extDoerDeployKey)(nil)

func (e *extDoerDeployKey) EncodeToString() string {
	return "deploy-key:" + strconv.FormatInt(e.DeployKeyID, 10)
}

func (e *extDoerDeployKey) DecodeFromString(s string) (err error) {
	idStr, _ := strings.CutPrefix(s, "deploy-key:")
	e.DeployKeyID, err = strconv.ParseInt(idStr, 10, 64)
	return err
}
