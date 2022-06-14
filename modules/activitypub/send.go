// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
)

func Fetch(iri *url.URL) (b []byte, err error) {
	req := httplib.NewRequest(iri.String(), http.MethodGet)
	req.Header("Accept", ActivityStreamsContentType)
	req.Header("Accept-Charset", "utf-8")
	req.Header("Date", fmt.Sprintf("%s UTC", time.Now().UTC().Format(time.RFC1123)))
	resp, err := req.Response()
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("url IRI fetch [%s] failed with status (%d): %s", iri, resp.StatusCode, resp.Status)
		return
	}
	b, err = io.ReadAll(resp.Body)
	return
}

func Send(user *user_model.User, activity *ap.Activity) {
	body, err := activity.MarshalJSON()
	if err != nil {
		return
	}

	for _, to := range activity.To {
		client, _ := NewClient(user, setting.AppURL+"api/v1/activitypub/user/"+user.Name+"#main-key")
		client.Post(body, to.GetID().String())
	}
}
