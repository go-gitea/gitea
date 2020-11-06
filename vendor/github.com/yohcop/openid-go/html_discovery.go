package openid

import (
	"errors"
	"io"

	"golang.org/x/net/html"
)

func htmlDiscovery(id string, getter httpGetter) (opEndpoint, opLocalID, claimedID string, err error) {
	resp, err := getter.Get(id, nil)
	if err != nil {
		return "", "", "", err
	}
	opEndpoint, opLocalID, err = findProviderFromHeadLink(resp.Body)
	return opEndpoint, opLocalID, resp.Request.URL.String(), err
}

func findProviderFromHeadLink(input io.Reader) (opEndpoint, opLocalID string, err error) {
	tokenizer := html.NewTokenizer(input)
	inHead := false
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			// Even if the document is malformed after we found a
			// valid <link> tag, ignore and let's be happy with our
			// openid2.provider and potentially openid2.local_id as well.
			if len(opEndpoint) > 0 {
				return
			}
			return "", "", tokenizer.Err()
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			tk := tokenizer.Token()
			if tk.Data == "head" {
				if tt == html.StartTagToken {
					inHead = true
				} else {
					if len(opEndpoint) > 0 {
						return
					}
					return "", "", errors.New(
						"LINK with rel=openid2.provider not found")
				}
			} else if inHead && tk.Data == "link" {
				provider := false
				localID := false
				href := ""
				for _, attr := range tk.Attr {
					if attr.Key == "rel" {
						if attr.Val == "openid2.provider" {
							provider = true
						} else if attr.Val == "openid2.local_id" {
							localID = true
						}
					} else if attr.Key == "href" {
						href = attr.Val
					}
				}
				if provider && !localID && len(href) > 0 {
					opEndpoint = href
				} else if !provider && localID && len(href) > 0 {
					opLocalID = href
				}
			}
		}
	}
	// At this point we should probably have returned either from
	// a closing </head> or a tokenizer error (no </head> found).
	// But just in case.
	if len(opEndpoint) > 0 {
		return
	}
	return "", "", errors.New("LINK rel=openid2.provider not found")
}
