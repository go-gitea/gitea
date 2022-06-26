// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

func Fetch(iri *url.URL) (b []byte, err error) {
	req := httplib.NewRequest(iri.String(), http.MethodGet)
	req.Header("Accept", ActivityStreamsContentType)
	req.Header("User-Agent", "Gitea/"+setting.AppVer)
	resp, err := req.Response()
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("url IRI fetch [%s] failed with status (%d): %s", iri, resp.StatusCode, resp.Status)
		return
	}
	b, err = io.ReadAll(io.LimitReader(resp.Body, setting.Federation.MaxSize))
	return b, err
}

func Send(user *user_model.User, activity *ap.Activity) {
	body, err := activity.MarshalJSON()
	if err != nil {
		return
	}
	var jsonmap map[string]interface{}
	err = json.Unmarshal(body, &jsonmap)
	if err != nil {
		return
	}
	jsonmap["@context"] = "https://www.w3.org/ns/activitystreams"
	body, _ = json.Marshal(jsonmap)

	for _, to := range activity.To {
		client, _ := NewClient(user, setting.AppURL+"api/v1/activitypub/user/"+user.Name+"#main-key")
		resp, _ := client.Post(body, to.GetID().String())
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, setting.Federation.MaxSize))
		log.Debug(string(respBody))
	}
}
