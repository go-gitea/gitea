package routers

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"gitea.com/macaron/csrf"
	"gitea.com/macaron/session"

	"code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/convert"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/structs"
)

type KitspaceSession struct {
	User *structs.User `json:"user"`
	Csrf string        `json:"_csrf"`
}

func Kitspace(ctx *context.Context, sess session.Store, x csrf.CSRF) (int, []byte) {
	url := ctx.Req.URL
	url.Scheme = "http"
	url.Host = "frontend:3000"
	url.Path = strings.Replace(
		ctx.Link,
		path.Join(setting.AppSubURL, "/__kitspace"),
		"",
		1,
	)
	var user *structs.User
	if (ctx.User != nil && ctx.IsSigned) {
		user = convert.ToUser(ctx.User, true, true)
	}

	m := KitspaceSession{
		User: user,
		Csrf: x.GetToken(),
	}

	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("GET", url.String(), bytes.NewBuffer(b))
	req.Header.Add("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	ctx.Resp.Header().Set("Content-Type", resp.Header.Get("Content-Type"))

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return resp.StatusCode, body
}
