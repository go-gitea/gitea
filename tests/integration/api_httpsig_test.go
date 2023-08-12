// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/tests"

	"github.com/go-fed/httpsig"
	"golang.org/x/crypto/ssh"
)

const (
	httpsigPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABFwAAAAdzc2gtcn
NhAAAAAwEAAQAAAQEAqjmQeb5Eb1xV7qbNf9ErQ0XRvKZWzUsLFhJzZz+Ab7q8WtPs91vQ
fBiypw4i8OTG6WzDcgZaV8Ndxn7iHnIstdA1k89MVG4stydymmwmk9+mrCMNsu5OmdIy9F
AZ61RDcKuf5VG2WKkmeK0VO+OMJIYfE1C6czNeJ6UAmcIOmhGxvjMI83XUO9n0ftwTwayp
+XU5prvKx/fTvlPjbraPNU4OzwPjVLqXBzpoXYhBquPaZYFRVyvfFZLObYsmy+BrsxcloM
l+9w4P0ATJ9njB7dRDL+RrN4uhhYSihqOK4w4vaiOj1+aA0eC0zXunEfLXfGIVQ/FhWcCy
5f72mMiKnQAAA9AxSmzFMUpsxQAAAAdzc2gtcnNhAAABAQCqOZB5vkRvXFXups1/0StDRd
G8plbNSwsWEnNnP4Bvurxa0+z3W9B8GLKnDiLw5MbpbMNyBlpXw13GfuIeciy10DWTz0xU
biy3J3KabCaT36asIw2y7k6Z0jL0UBnrVENwq5/lUbZYqSZ4rRU744wkhh8TULpzM14npQ
CZwg6aEbG+MwjzddQ72fR+3BPBrKn5dTmmu8rH99O+U+Nuto81Tg7PA+NUupcHOmhdiEGq
49plgVFXK98Vks5tiybL4GuzFyWgyX73Dg/QBMn2eMHt1EMv5Gs3i6GFhKKGo4rjDi9qI6
PX5oDR4LTNe6cR8td8YhVD8WFZwLLl/vaYyIqdAAAAAwEAAQAAAQBz+nyBNi2SYir6SxPA
flcnoq5gBkUl4ndPNosCUbXEakpi5/mQHzJRGtK+F1efIYCVEdGoIsPy/90onNKbQ9dKmO
2oI5kx/U7iCzJ+HCm8nqkEp21x+AP9scWdx+Wg/OxmG8j5iU7f4X+gwOyyvTqCuA78Lgia
7Oi9wiJCoIEqXr6dRYGJzfASwKA2dj995HzATexleLSD5fQCmZTF+Vh5OQ5WmE+c53JdZS
T3Plie/P/smgSWBtf1fWr6JL2+EBsqQsIK1Jo7r/7rxsz+ILoVfnneNQY4QSa9W+t6ZAI+
caSA0Guv7vC92ewjlMVlwKa3XaEjMJb5sFlg1r6TYMwBAAAAgQDQwXvgSXNaSHIeH53/Ab
t4BlNibtxK8vY8CZFloAKXkjrivKSlDAmQCM0twXOweX2ScPjE+XlSMV4AUsv/J6XHGHci
W3+PGIBfc/fQRBpiyhzkoXYDVrlkSKHffCnAqTUQlYkhr0s7NkZpEeqPE0doAUs4dK3Iqb
zdtz8e5BPXZwAAAIEA4U/JskIu5Oge8Is2OLOhlol0EJGw5JGodpFyhbMC+QYK9nYqy7wI
a6mZ2EfOjjwIZD/+wYyulw6cRve4zXwgzUEXLIKp8/H3sYvJK2UMeP7y68sQFqGxbm6Rnh
tyBBSaJQnOXVOFf9gqZGCyO/J0Illg3AXTuC8KS/cxwasC38EAAACBAMFo/6XQoR6E3ynj
VBaz2SilWqQBixUyvcNz8LY73IIDCecoccRMFSEKhWtvlJijxvFbF9M8g9oKAVPuub4V5r
CGmwVPEd5yt4C2iyV0PhLp1PA2/i42FpCSnHaz/EXSz6ncTZcOMMuDqUbgUUpQg4VSUDl9
fhTNAzWwZoQ91aHdAAAAFHUwMDIyMTQ2QGljdHMtcC1ueC03AQIDBAUG
-----END OPENSSH PRIVATE KEY-----`
	httpsigCertificate = `ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1yc2EtY2VydC12MDFAb3BlbnNzaC5jb20AAAAgiR7SU8gmZLhopx4Y03nOXVuAb+4fyMcJYjMGcE1Z2oEAAAADAQABAAABAQCqOZB5vkRvXFXups1/0StDRdG8plbNSwsWEnNnP4Bvurxa0+z3W9B8GLKnDiLw5MbpbMNyBlpXw13GfuIeciy10DWTz0xUbiy3J3KabCaT36asIw2y7k6Z0jL0UBnrVENwq5/lUbZYqSZ4rRU744wkhh8TULpzM14npQCZwg6aEbG+MwjzddQ72fR+3BPBrKn5dTmmu8rH99O+U+Nuto81Tg7PA+NUupcHOmhdiEGq49plgVFXK98Vks5tiybL4GuzFyWgyX73Dg/QBMn2eMHt1EMv5Gs3i6GFhKKGo4rjDi9qI6PX5oDR4LTNe6cR8td8YhVD8WFZwLLl/vaYyIqdAAAAAAAAAAEAAAABAAAABXVzZXIxAAAACQAAAAV1c2VyMQAAAABimoIOAAAAAMCWkRMAAAAAAAAAggAAABVwZXJtaXQtWDExLWZvcndhcmRpbmcAAAAAAAAAF3Blcm1pdC1hZ2VudC1mb3J3YXJkaW5nAAAAAAAAABZwZXJtaXQtcG9ydC1mb3J3YXJkaW5nAAAAAAAAAApwZXJtaXQtcHR5AAAAAAAAAA5wZXJtaXQtdXNlci1yYwAAAAAAAAAAAAABlwAAAAdzc2gtcnNhAAAAAwEAAQAAAYEAm+AwtXTBZyeqV1qOxjMU3Ibc5iR2M3zerGfRQDxUeIozC3xpIvqJbzjDuRapdf8hpxn2xC0GtUusuLIUr4/+Svs1BUnJhF2H9xnK/O0aopS5MpNekUvnBzQdbvO8Ux2xE2mt58giXhkEaXeCEODSqG++OZsA2e40AR/AGRJ4OdDofMvH4vLJAQQc2mKdYpYL8xu+NC+7nsenx1etpsqtEl3gmvqCVI6t9uhVPMvlbGt9h/AN3u7ToF2T3bdk1TZbcdkvR9ljvETIuy32ksAETX8tc7vm30edK+nn/GMeWCgjM+MFm9Uh1NRkvNNJozo5SJy0DkWETTJUsEdfry5VQ3IjqhWqQ0m4/mDlTmsEdEdWqpUiqWZLd9w7jgT8fanuglZyIu2fj8fyqjPjiws5S2P0Uvi28UKQ1nH01UYj/kuakU3BNzN1IqDf3tARP9fjKV/dCBqb1ZAOtyC2GyhGuGzNwEi+woUwq+sTeV0/hqVSb3hSitXHzcfRMRyOK82BAAABlAAAAAxyc2Etc2hhMi01MTIAAAGAMBfgZFvz4BdxriGKYd6eRhMo6hf+I8S9uzNRsflJXHuA+HR9ExIm/Q9JjKmfThQzNyGGBOBILaDU205SAJuG+kk3SieSQDd75ZQd8YmNlCc+516AriOsTiyVCupnf3I2euTjMZqEZbJcBbkBljppTOWQVN7xxE8QakDfGhg0+RjJE9wYOTmkKpDBfII5Nw8V5DoOD7kNEpXYqHdy/8lVxpqUYNIP1J0dNP4f6qBcZcM1PDA12q8zwIGqSNNjf2UXY/Nr8nv9CnK4fB8NDOPKTBa4cm48BGbvM/X0l6dYKswuZ9Np8lw+y6+GxTgznGCrkzMmuEV4FzSq4xHp41H2L2MTwUkwYaeyG1VP6aWkvn6zPkSxaaJDfQX7CAFe17IhIGXR0UPLjKjh35nDLzMWb/W6/W1lK9YkZNHXSf7Z9m9MUAZN7yQgOggGsuYEW4imZxvZizMd+fdDu9mbhr0FDis89I7MSJDnyYRE9FXS7p3QpppBwGcss/9yV3JV3Bjc`
)

func TestHTTPSigPubKey(t *testing.T) {
	// Add our public key to user1
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")
	token := url.QueryEscape(getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser))
	keysURL := fmt.Sprintf("/api/v1/user/keys?token=%s", token)
	keyType := "ssh-rsa"
	keyContent := "AAAAB3NzaC1yc2EAAAADAQABAAABAQCqOZB5vkRvXFXups1/0StDRdG8plbNSwsWEnNnP4Bvurxa0+z3W9B8GLKnDiLw5MbpbMNyBlpXw13GfuIeciy10DWTz0xUbiy3J3KabCaT36asIw2y7k6Z0jL0UBnrVENwq5/lUbZYqSZ4rRU744wkhh8TULpzM14npQCZwg6aEbG+MwjzddQ72fR+3BPBrKn5dTmmu8rH99O+U+Nuto81Tg7PA+NUupcHOmhdiEGq49plgVFXK98Vks5tiybL4GuzFyWgyX73Dg/QBMn2eMHt1EMv5Gs3i6GFhKKGo4rjDi9qI6PX5oDR4LTNe6cR8td8YhVD8WFZwLLl/vaYyIqd"
	rawKeyBody := api.CreateKeyOption{
		Title: "test-key",
		Key:   keyType + " " + keyContent,
	}
	req := NewRequestWithJSON(t, "POST", keysURL, rawKeyBody)
	MakeRequest(t, req, http.StatusCreated)

	// parse our private key and create the httpsig request
	sshSigner, _ := ssh.ParsePrivateKey([]byte(httpsigPrivateKey))
	keyID := ssh.FingerprintSHA256(sshSigner.PublicKey())

	// create the request
	token = getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeReadAdmin)
	req = NewRequest(t, "GET", fmt.Sprintf("/api/v1/admin/users?token=%s", token))

	signer, _, err := httpsig.NewSSHSigner(sshSigner, httpsig.DigestSha512, []string{httpsig.RequestTarget, "(created)", "(expires)"}, httpsig.Signature, 10)
	if err != nil {
		t.Fatal(err)
	}

	// sign the request
	err = signer.SignRequest(keyID, req, nil)
	if err != nil {
		t.Fatal(err)
	}

	// make the request
	MakeRequest(t, req, http.StatusOK)
}

func TestHTTPSigCert(t *testing.T) {
	// Add our public key to user1
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user1")

	csrf := GetCSRF(t, session, "/user/settings/keys")
	req := NewRequestWithValues(t, "POST", "/user/settings/keys", map[string]string{
		"_csrf":   csrf,
		"content": "user1",
		"title":   "principal",
		"type":    "principal",
	})

	session.MakeRequest(t, req, http.StatusSeeOther)
	pkcert, _, _, _, err := ssh.ParseAuthorizedKey([]byte(httpsigCertificate))
	if err != nil {
		t.Fatal(err)
	}

	// parse our private key and create the httpsig request
	sshSigner, _ := ssh.ParsePrivateKey([]byte(httpsigPrivateKey))
	keyID := "gitea"

	// create our certificate signer using the ssh signer and our certificate
	certSigner, err := ssh.NewCertSigner(pkcert.(*ssh.Certificate), sshSigner)
	if err != nil {
		t.Fatal(err)
	}

	// create the request
	req = NewRequest(t, "GET", "/api/v1/admin/users")

	// add our cert to the request
	certString := base64.RawStdEncoding.EncodeToString(pkcert.(*ssh.Certificate).Marshal())
	req.Header.Add("x-ssh-certificate", certString)

	signer, _, err := httpsig.NewSSHSigner(certSigner, httpsig.DigestSha512, []string{httpsig.RequestTarget, "(created)", "(expires)", "x-ssh-certificate"}, httpsig.Signature, 10)
	if err != nil {
		t.Fatal(err)
	}

	// sign the request
	err = signer.SignRequest(keyID, req, nil)
	if err != nil {
		t.Fatal(err)
	}

	// make the request
	MakeRequest(t, req, http.StatusOK)
}
