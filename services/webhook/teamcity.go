package webhook

import (
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/log"
	api "code.gitea.io/gitea/modules/structs"
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

// GetTeamCityPayload returns the payload as-is
// TeamCity requests API doesn't take a body on POST, so no need to alter it in any way.
func GetTeamCityPayload(p api.Payloader, event webhook_model.HookEventType, meta string) (api.Payloader, error) {
	return p, nil
}

// GetTeamCityHook returns TeamCity metadata
func GetTeamCityHook(w *webhook_model.Webhook) *TeamCityMeta {
	s := &TeamCityMeta{}
	if err := json.Unmarshal([]byte(w.Meta), s); err != nil {
		log.Error("webhook.GetTeamCityHook(%d): %v", w.ID, err)
	}
	return s
}
