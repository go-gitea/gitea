package webhook

import (
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/log"
	"encoding/json"
)

type (

	// TeamCityMeta contains metadata for the TeamCity WebHook
	TeamCityMeta struct {
		HostUrl   string `json:"host_url"`
		AuthToken string `json:"auth_token"`
		VcsRootId string `json:"vcs_root_id"`
	}
)

// GetTeamCityHook returns TeamCity metadata
func GetTeamCityHook(w *webhook_model.Webhook) *TeamCityMeta {
	s := &TeamCityMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetTeamCityHook(%d): %v", w.ID, err)
	}
	return s
}
