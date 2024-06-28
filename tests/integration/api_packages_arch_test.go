// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models/db"
	packages "code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	arch_model "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageArch(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	unPack := func(s string) []byte {
		data, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "\n", ""), "\r", ""))
		return data
	}
	rootURL := fmt.Sprintf("/api/packages/%s/arch", user.Name)

	pkgs := map[string][]byte{
		"any": unPack(`
KLUv/QBYXRMABmOHSbCWag6dY6d8VNtVR3rpBnWdBbkDAxM38Dj3XG3FK01TCKlWtMV9QpskYdsm
e6fh5gWqM8edeurYNESoIUz/RmtyQy68HVrBj1p+AIoAYABFSJh4jcDyWNQgHIKIuNgIll64S4oY
FFIUk6vJQBMIIl2iYtIysqKWVYMCYvXDpAKTMzVGwZTUWhbciFCglIMH1QMbEtjHpohSi8XRYwPr
AwACSy/fzxO1FobizlP7sFgHcpx90Pus94Edjcc9GOustbD3PBprLUxH50IGC1sfw31c7LOfT4Qe
nh0KP1uKywwdPrRYmuyIkWBHRlcLfeBIDpKKqw44N0K2nNAfFW5grHRfSShyVgaEIZwIVVmFGL7O
88XDE5whJm4NkwA91dRoPBCcrgqozKSyah1QygsWkCshAaYrvbHCFdUTJCOgBpeUTMuJJ6+SRtcj
wIRua8mGJyg7qWoqJQq9z/4+DU1rHrEO8f6QZ3HUu3IM7GY37u+jeWjUu45637yN+qj338cdi0Uc
y0a9a+e5//1cYnPUu37dxr15khzNQ9/PE80aC/1okjz9mGo3bqP5Ue+scflGshdzx2g28061k2PW
uKwzjmV/XzTzzmKdcfz3eRbJoRPddcaP/n4PSZqQeYa1PDtPQzOHJK0amfjvz0IUV/v38xHJK/rz
JtFpalPD30drDWi7Bl8NB3J/P3csijQyldWZ8gy3TNslLsozMw74DhoAXoAfnE8xydUUHPZ3hML4
2zVDGiEXSGYRx4BKQDcDJA5S9Ca25FRgPtSWSowZJpJTYAR9WCPHUDgACm6+hBecGDPNClpwHZ2A
EQ==
`),
		"x86_64": unPack(`
KLUv/QBYnRMAFmOJS7BUbg7Un8q21hxCopsOMn6UGTzJRbHI753uOeMdxZ+V7ajoETVxl9CSBCR5
2a3K1vr1gwyp9gCTH422bRNxHEg7Z0z9HV4rH/DGFn8AjABjAFQ2oaUVMRRGViVoqmxAVKuoKQVM
NJRwTDl9NcHCClliWjTpWin6sRUZsXSipWlAipQnleThRgFF5QTAzpth0UPFkhQeJRnYOaqSScEC
djCPDwE8pQTfVXW9F7bmznX3YTNZDeP7IHgxDazNQhp+UDa798KeRgvvvbCamgsYdL461TfvcmlY
djFowWYH5yaH5ztZcemh4omAkm7iQIWvGypNIXJQNgc7DVuHjx06I4MZGTIkeEBIOIL0OxcvnGps
0TwxycqKYESrwwQYEDKI2F0hNXH1/PCQ2BS4Ykki48EAaflAbRHxYrRQbdAZ4oXVAMGCkYOXkBRb
NkwjNCoIF07ByTlyfJhmoHQtCbFYDN+941783KqzusznmPePXJPluS1+cL/74Rd/1UHluW15blFv
ol6e+8XPPZNDPN/Kc9vOdX/xNZrT8twWnH34U9Xkqw76rqqrPjPQl6nJde9i74e/8Mtz6zOjT3R7
Uve8BrabpT4zanE83158MtVbkxbH84vPNWkGqeu2OF704vfRzAGl6mhRtXPdmOrRzFla+BO+DL34
uHHN9r74usjkduX5VEhNz9TnxV9trSabvYAwuIZffN0zSeZM3c3GUHX8dG6jeUgHGgBbgB9cUDHJ
1RR09teBwvjbNUMaIRdIZhHHgEpANwMkDpL0JsbkVFA+0JZKjBkmklNgBH1YI8dQOAAKbr6EF5wY
M80KWnAdnYAR
`),
		"aarch64": unPack(`
KLUv/QBYRRQAVmSLSbCWag6dY6d8VNtVR3rpBnWdBbkDAxM38Dj3XG3FK01TCKlWtKXyhOyOAkVM
tttyu2KP3HC0f/cR1ERIUxznCyzBnRiwcCQO/3doAGkAfQBFHAeRpq6d5/7hZxo+SVPXr+Pun2jJ
dx76jqI7m1joybPk6cdUy10u9qSps4lpbxT0Y+545zPrfMxvJ0riPVq7bdx5utnfug+EXfvfBa33
gV3Mu3/vZz/wWru958WstVtPBJ4dHQo+XGcWy1n1YjJxY/zhy8SNq/3w81HJ6/b5kwg9zVPDh78r
DWi7Bh/NBe+Hnzsex1mZBtUZGipyeaZpuCjPzDjgunfZhz/bac0kNkWsX+R5JE1dLwPLWe5++DsP
kaaupKnDrI06aeoPH3c8BilgUEhRTq4mA00giHSJiknLyIpaVg0KiJUPkwoMzhQZtSmptSywEaFA
KQcPqgc1JHBPTRGlFouDxwbWBwAEll46im5YB95rH/Q+ENbl5FzIYIGrvSSziWlVxnHQhz+eWeex
yjj+8HkeyyF06yrjRx9+F8uZ0ImKlYZ2nooBKS4zdPjQYmmyI0YCHRldLcwDR3CQVFx1uLERsuW0
f1SwgbHSfSWhwFkZIIZwIlRlJWL4QtEXD09uhpgIq5jk54mmNtuB4HRVQEUmlVXrflJesIC3hASY
rvTGClcUD5CMgBpcUjItJ568aDS6HgEmdFtLNjxB0UlVUylRERImXiOwPBY1CAcgIi40gqUX7hIj
IOACGSLqYpPAD5iSNlT2MJRJwREAF4FRHPBlCJMQPRcwaGAsDJA2+KIArkIJGNtCydULTuN1oBh/
+zKkEblAsgjGqVgUwKLP+UOMOGCpAhICtg6ncFJH`),
	}

	t.Run("RepositoryKey", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", rootURL+"/repository.key")
		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, "application/pgp-keys", resp.Header().Get("Content-Type"))
		assert.Contains(t, resp.Body.String(), "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	})

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["any"]))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["any"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeArch)
		assert.NoError(t, err)
		assert.Len(t, pvs, 1)

		pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
		assert.NoError(t, err)
		assert.Nil(t, pd.SemVer)
		assert.IsType(t, &arch_model.VersionMetadata{}, pd.Metadata)
		assert.Equal(t, "test", pd.Package.Name)
		assert.Equal(t, "1.0.0-1", pd.Version.Version)

		pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
		assert.NoError(t, err)
		assert.Len(t, pfs, 2) // zst and zst.sig
		assert.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		assert.NoError(t, err)
		assert.Equal(t, int64(len(pkgs["any"])), pb.Size)

		req = NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["any"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusConflict)
		req = NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["x86_64"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithBody(t, "PUT", rootURL+"/other", bytes.NewReader(pkgs["any"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
	})

	t.Run("Download", func(t *testing.T) {
	})

	t.Run("Sign", func(t *testing.T) {
	})

	t.Run("Database", func(t *testing.T) {
	})

	t.Run("Delete", func(t *testing.T) {
	})
}
