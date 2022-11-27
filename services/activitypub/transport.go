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
	"code.gitea.io/gitea/modules/forgefed"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
	"github.com/go-ap/jsonld"
)

// Fetch a remote ActivityStreams object
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

// Send an activity
func Send(user *user_model.User, activity *ap.Activity) error {
	binary, err := jsonld.WithContext(
		jsonld.IRI(ap.ActivityBaseURI),
		jsonld.IRI(ap.SecurityContextURI),
		jsonld.IRI(forgefed.ForgeFedNamespaceURI),
	).Marshal(activity)
	if err != nil {
		return err
	}

	for _, to := range activity.To {
		client, _ := NewClient(user, setting.AppURL+"api/v1/activitypub/user/"+user.Name+"#main-key")
		resp, _ := client.Post(binary, to.GetLink().String())
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, setting.Federation.MaxSize))
		log.Trace("Response from sending activity", string(respBody))
	}
	return err
}
