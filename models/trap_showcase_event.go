package models

import (
	"os"
	"net/http"
	"net/url"
	"time"
)

var instances = []string{
	"https://showcase.trapti.tech",
	"https://cpctf.showcase.trapti.tech",
}

var traP_token = os.Getenv("SHOWCASE_TRAP_TOKEN")

func ShowcaseNotify(targets []string, endpoint string, v url.Values) {
	if (traP_token == "") {
		return
	}
	time.Sleep(1 * time.Second)
	client := &http.Client{}

	for _, url := range targets {
		req, _ := http.NewRequest("GET", url+"/api"+endpoint+"?"+v.Encode(), nil)

		req.AddCookie(&http.Cookie{
			Name:  "traP_token",
			Value: traP_token,
		})

		client.Do(req)
	}
}

func ShowcasePushEvent(owner, repo, ref string) {
	v := url.Values{}
	v.Add("repo", owner+"/"+repo)
	v.Add("ref", ref)

	target := 0
	if owner == "CPCTF2018" {
		target = 1
	}

	go ShowcaseNotify([]string{instances[target]}, "/create", v)
}

func ShowcaseKeyUpdateEvent(ownerID int64) {
	user, _ := GetUserByID(ownerID)
	v := url.Values{}
	v.Add("name", user.Name)
	go ShowcaseNotify(instances, "/update_key", v)
}
