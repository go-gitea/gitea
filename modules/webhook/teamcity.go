package webhook

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/log"
	"encoding/json"
)

type (
	TeamCityMeta struct {
		HostUrl   string `json:"host_url"`
		AuthToken string `json:"auth_token"`
		VcsRootId string `json:"vcs_root_id"`
	}
)

func GetTeamCityHook(w *models.Webhook) *TeamCityMeta {
	s := &TeamCityMeta{}

	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetTeamCityHook(%d): %v", w.ID, err)
	}

	return s
}
