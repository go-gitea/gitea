package openid

import (
	"net/http"
	"net/url"
)

// Interface that simplifies testing.
type httpGetter interface {
	Get(uri string, headers map[string]string) (resp *http.Response, err error)
	Post(uri string, form url.Values) (resp *http.Response, err error)
}

type defaultGetter struct {
	client *http.Client
}

func (dg *defaultGetter) Get(uri string, headers map[string]string) (resp *http.Response, err error) {
	request, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return
	}
	for h, v := range headers {
		request.Header.Add(h, v)
	}
	return dg.client.Do(request)
}

func (dg *defaultGetter) Post(uri string, form url.Values) (resp *http.Response, err error) {
	return dg.client.PostForm(uri, form)
}
