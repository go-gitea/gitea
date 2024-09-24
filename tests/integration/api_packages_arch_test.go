// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/packages"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	arch_model "code.gitea.io/gitea/modules/packages/arch"
	"code.gitea.io/gitea/tests"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/require"
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
KLUv/QBYdRQAVuSMS7BUbg7Un8q21hxCopsOMn6UGTzJRbHI753uOeMdxZ+V7ajoEbUkUXbXhXW/
7FanWzv7B/EcMxhodFqyZkUcB9LOGVN/h9MqG7zFFmoAaQB8AEFrvpXntn3V/cXXaE7Lc9uP5uFP
VXPl+ue7qnJ9Zp8vU3PVvYu9HvbAL8+tz4y+0O1J3TPXqbZ5l3+lapk5ee+L577qXvdf+Atn+P69
4Qz8QhpYw4/xd78Q3/v6Wg28974u1Ojc2ODseAGpHs2crYG4kef84uNGnu198fWQuVq+8ymQmp5p
z4vPbRjOaBC+FxziF1/3TJI5U3ezMlQdPZ3baA7SMhnMunvHvfg5rrO6zOeY94+rJstzW/zgetfD
Lz7XP+W5bXluUW+hXp77xc89kwFRTF1PrKxAFpgXT7ZWhjzYjpRIStGyNCAGBYM6AnGrkKKCAmAH
k3HBI8VyBBYdGdApmoqJYQE62EeIADCkBF1VOW0WYnz/+y6ufTMaDQ2GDDme7Wapz4xa3JpvLz6Z
6q1Ji1vzi79q0vxR+ba4dejF76OZ80nV0aJqX3VjKCsuP1g0EWDSURyw0JVDZWlEzsnmYLdh8wDS
I2dkIEMjxsSOiAlJjH4HIwbTjayZJidXVxKQYH2gICOCBhK7KqMlLZ4gMCU1BapYlsTAXnywepyy
jMBmtEhxyCnCZdUAwYKxAxeRFVk4TCL0aYgWjt3kHTg9SjVStppI2YCSWshUEFGdmJmyCVGpnqIU
KNlA0hEjIOACGSLqYpXAD5SSNVT2MJRJwREAF4FRHPBlCJMSNwFguGAWDJBg+KIArkIJGNtCydUL
TuN1oBh/+zKkEblAsgjGqVgUwKLP+UOMOGCpAhICtg6ncFJH`),
		"otherXZ": unPack(`
/Td6WFoAAATm1rRGBMCyBIAYIQEWAAAAAAAAABaHRszgC/8CKl0AFxNGhTWwfXmuDQEJlHgNLrkq
VxpJY6d9iRTt6gB4uCj0481rnYfXaUADHzOFuF3490RPrM6juPXrknqtVyuWJ5efW19BgwctN6xk
UiXiZaXVAWVWJWy2XHJiyYCMWBfIjUfo1ccOgwolwgFHJ64ZJjbayA3k6lYPcImuAqYL5NEVHpwl
Z8CWIjiXXSMQGsB3gxMdq9nySZbHQLK/KCKQ+oseF6kXyIgSEyuG4HhjVBBYIwTvWzI06kjNUXEy
2sw0n50uocLSAwJ/3mdX3n3XF5nmmuQMPtFbdQgQtC2VhyVd3TdIF+pT6zAEzXFJJ3uLkNbKSS88
ZdBny6X/ftT5lQpNi/Wg0xLEQA4m4fu4fRAR0kOKzHM2svNLbTxa/wOPidqPzR6b/jfKmHkXxBNa
jFafty0a5K2S3F6JpwXZ2fqti/zG9NtMc+bbuXycC327EofXRXNtuOupELDD+ltTOIBF7CcTswyi
MZDP1PBie6GqDV2GuPz+0XXmul/ds+XysG19HIkKbJ+cQKp5o7Y0tI7EHM8GhwMl7MjgpQGj5nuv
0u2hqt4NXPNYqaMm9bFnnIUxEN82HgNWBcXf2baWKOdGzPzCuWg2fAM4zxHnBWcimxLXiJgaI8mU
J/QqTPWE0nJf1PW/J9yFQVR1Xo0TJyiX8/ObwmbqUPpxRGjKlYRBvn0jbTdUAENBSn+QVcASRGFE
SB9OM2B8Bg4jR/oojs8Beoq7zbIblgAAAACfRtXvhmznOgABzgSAGAAAKklb4rHEZ/sCAAAAAARZ
Wg==`),
		"otherZST": unPack(`
KLUv/QRYbRMABuOHS9BSNQdQ56F+xNFoV3CijY54JYt3VqV1iUU3xmj00y2pyBOCuokbhDYpvNsj
ZJeCxqH+nQFpMf4Wa92okaZoF4eH6HsXXCBo+qy3Fn4AigBgAEaYrLCQEuAom6YbHyuKZAFYksqi
sSOFiRs0WDmlACk0CnpnaAeKiCS3BlwVkViJEbDS43lFNbLkZEmGhc305Nn4AMLGiUkBDiMTG5Vz
q4ZISjCofEfR1NpXijvP2X95Hu1e+zLalc0+mjeT3Z/FPGvt62WymbX2dXMDIYKDLjjP8n03RrPf
A1vOApwGOh2MgE2LpgZrgXLDF2CUJ15idG2J8GCSgcc2ZVRgA8+RHD0k2VJjg6mRUgGGhBWEyEcz
5EePLhUeWlYhoFCKONxUiBiIUiQeDIqiQwkjLiyqnF5eGs6a2gGRapbU9JRyuXAlPemYajlJojJd
GBBJjo5GxFRkITOAvLhSCr2TDz4uzdU8Yh3i/SHP4qh3vTG2s9198NP8M+pdR73BvIP6qPeDjzsW
gTi+jXrXWOe5P/jZxOeod/287v6JljzNP99RNM0a+/x4ljz3LNV2t5v9qHfW2Pyg24u54zSfObWX
Y9bYrCTHtwdfPPPOYiU5fvB5FssfNN2V5EIPfg9LnM+JhtVEO8+FZw5LXA068YNPhimu9sHPQiWv
qc6fE9BTnxIe/LTKatab+WYu7T74uWNRxJW5W5Ux0bDLuG1ioCwjg4DvGgBcgB8cUDHJ1RQ89neE
wvjbNUMiIZdo5hbHgEpANwMkDnL0Jr7kVFg+0pZKjBkmklNgBH1YI8dQOAAKbr6EF5wYM80KWnAd
nYARrByncQ==`),
		"otherGZ": unPack(`
H4sIAAAAAAAAA9PzDQlydWWgKTAwMDAzMVEA0UCAThsYGBuZKRiamBmbm5qZGJqbKBgYGpobGzMo
GNDWWRBQWlySWAR0SlF+fgk+dYTk0T03RIB8NweEwVx71tDviIFA60O75Rtc5s+9YbxteUHzhUWi
HBkWDcbGcUqCukrLGi4Lv8jIqNsbXhueXW8uzTe79Lr9/TVbnl69c3wR652f21+7rnU5kmjTc/38
8t+zLx/+ePFr6lajpZ2dzCkyB3NPTxdVOfFk2/RXmq+Ktq2dbnY6RcPCMW8Kg9aGszs1f6+YsTlf
x5j5eIpXnXzStAbJvQvPP3su//3lu2/2pj++XO9hbJS+puPmqJKREff4X+RUqdYTbpGTBGYuefH9
mNbGzKNdiUmS+xgt7J+5iTMObIgOLaAX4O3u6efmT0s7COV/UwNztPxvZGhqOpr/6QGUFdxT81KL
EktSUxSSKhVyE7NTC7LTFcz0DPUMuJQVSosz89IV0oCiIP8rlKUWFWfm5ykY6hmbcgHV5SXmpirY
KpSkFpcYgfhJicUIfkVKYkkikAcUL6ksSLUF0iA1QDOAgkDj9Qx0DUECKanFyVBNCgWJydmJ6alc
pUU5QKGMkpKCYit9/dSKxNyCnFS95Pxcfa6k0sycFKDRIIsMzQ0tTS2NDSxMuKA6QWaH5mXn5Zfn
KQRAhbiKM6tAqg24EouSM4CMxLxKrpzM5NQ8sGuTgUkgP5crOT8vDShYAhSpKs7gKijKL8sEOg2k
HMhNSS1IzUsBcpJAPFAwwUXSM0u4BjoaR8EoGAWjgGQAAILFeyQADAAA
`),
	}

	t.Run("RepositoryKey", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", rootURL+"/repository.key")
		resp := MakeRequest(t, req, http.StatusOK)

		require.Equal(t, "application/pgp-keys", resp.Header().Get("Content-Type"))
		require.Contains(t, resp.Body.String(), "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	})

	for _, group := range []string{"", "arch", "arch/os", "x86_64"} {
		groupURL := rootURL
		if group != "" {
			groupURL = groupURL + "/" + group
		}
		t.Run(fmt.Sprintf("Upload[%s]", group), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()

			req := NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs["any"]))
			MakeRequest(t, req, http.StatusUnauthorized)

			req = NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs["any"])).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			req = NewRequestWithBody(t, "PUT", groupURL, bytes.NewBuffer([]byte("any string"))).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusBadRequest)

			pvs, err := packages.GetVersionsByPackageType(db.DefaultContext, user.ID, packages.TypeArch)
			require.NoError(t, err)
			require.Len(t, pvs, 1)

			pd, err := packages.GetPackageDescriptor(db.DefaultContext, pvs[0])
			require.NoError(t, err)
			require.Nil(t, pd.SemVer)
			require.IsType(t, &arch_model.VersionMetadata{}, pd.Metadata)
			require.Equal(t, "test", pd.Package.Name)
			require.Equal(t, "1.0.0-1", pd.Version.Version)

			pfs, err := packages.GetFilesByVersionID(db.DefaultContext, pvs[0].ID)
			require.NoError(t, err)
			size := 0
			for _, pf := range pfs {
				if pf.CompositeKey == group {
					size++
				}
			}
			require.Equal(t, 2, size) // zst and zst.sig

			pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
			require.NoError(t, err)
			require.Equal(t, int64(len(pkgs["any"])), pb.Size)

			req = NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs["any"])).
				AddBasicAuth(user.Name) // exists
			MakeRequest(t, req, http.StatusConflict)
			req = NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs["x86_64"])).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs["aarch64"])).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)
			req = NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs["aarch64"])).
				AddBasicAuth(user.Name) // exists again
			MakeRequest(t, req, http.StatusConflict)
		})

		t.Run(fmt.Sprintf("Download[%s]", group), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequest(t, "GET", groupURL+"/x86_64/test-1.0.0-1-x86_64.pkg.tar.zst")
			resp := MakeRequest(t, req, http.StatusOK)
			require.Equal(t, pkgs["x86_64"], resp.Body.Bytes())

			req = NewRequest(t, "GET", groupURL+"/x86_64/test-1.0.0-1-any.pkg.tar.zst")
			resp = MakeRequest(t, req, http.StatusOK)
			require.Equal(t, pkgs["any"], resp.Body.Bytes())

			// get other group
			req = NewRequest(t, "GET", rootURL+"/unknown/x86_64/test-1.0.0-1-aarch64.pkg.tar.zst")
			MakeRequest(t, req, http.StatusNotFound)
		})

		t.Run(fmt.Sprintf("SignVerify[%s]", group), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequest(t, "GET", rootURL+"/repository.key")
			respPub := MakeRequest(t, req, http.StatusOK)

			req = NewRequest(t, "GET", groupURL+"/x86_64/test-1.0.0-1-any.pkg.tar.zst")
			respPkg := MakeRequest(t, req, http.StatusOK)

			req = NewRequest(t, "GET", groupURL+"/x86_64/test-1.0.0-1-any.pkg.tar.zst.sig")
			respSig := MakeRequest(t, req, http.StatusOK)

			if err := gpgVerify(respPub.Body.Bytes(), respSig.Body.Bytes(), respPkg.Body.Bytes()); err != nil {
				t.Fatal(err)
			}
		})

		t.Run(fmt.Sprintf("RepositoryDB[%s]", group), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			req := NewRequest(t, "GET", rootURL+"/repository.key")
			respPub := MakeRequest(t, req, http.StatusOK)

			req = NewRequest(t, "GET", groupURL+"/x86_64/base.db")
			respPkg := MakeRequest(t, req, http.StatusOK)

			req = NewRequest(t, "GET", groupURL+"/x86_64/base.db.sig")
			respSig := MakeRequest(t, req, http.StatusOK)

			if err := gpgVerify(respPub.Body.Bytes(), respSig.Body.Bytes(), respPkg.Body.Bytes()); err != nil {
				t.Fatal(err)
			}
			files, err := listTarGzFiles(respPkg.Body.Bytes())
			require.NoError(t, err)
			require.Len(t, files, 1)
			for s, d := range files {
				name := getProperty(string(d.Data), "NAME")
				ver := getProperty(string(d.Data), "VERSION")
				require.Equal(t, name+"-"+ver+"/desc", s)
				fn := getProperty(string(d.Data), "FILENAME")
				pgp := getProperty(string(d.Data), "PGPSIG")
				req = NewRequest(t, "GET", groupURL+"/x86_64/"+fn+".sig")
				respSig := MakeRequest(t, req, http.StatusOK)
				decodeString, err := base64.StdEncoding.DecodeString(pgp)
				require.NoError(t, err)
				require.Equal(t, respSig.Body.Bytes(), decodeString)
			}
		})

		t.Run(fmt.Sprintf("Delete[%s]", group), func(t *testing.T) {
			defer tests.PrintCurrentTest(t)()
			// test data
			req := NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs["otherXZ"])).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusCreated)

			req = NewRequestWithBody(t, "DELETE", rootURL+"/base/notfound/1.0.0-1/any", nil).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequestWithBody(t, "DELETE", groupURL+"/test/1.0.0-1/x86_64", nil).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequestWithBody(t, "DELETE", groupURL+"/test/1.0.0-1/any", nil).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequest(t, "GET", groupURL+"/x86_64/base.db")
			respPkg := MakeRequest(t, req, http.StatusOK)
			files, err := listTarGzFiles(respPkg.Body.Bytes())
			require.NoError(t, err)
			require.Len(t, files, 1)

			req = NewRequestWithBody(t, "DELETE", groupURL+"/test2/1.0.0-1/any", nil).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequest(t, "GET", groupURL+"/x86_64/base.db").
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNotFound)

			req = NewRequestWithBody(t, "DELETE", groupURL+"/test/1.0.0-1/aarch64", nil).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)

			req = NewRequest(t, "GET", groupURL+"/aarch64/base.db").
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNotFound)
		})

		for tp, key := range map[string]string{
			"GZ":  "otherGZ",
			"XZ":  "otherXZ",
			"ZST": "otherZST",
		} {
			t.Run(fmt.Sprintf("Upload%s[%s]", tp, group), func(t *testing.T) {
				defer tests.PrintCurrentTest(t)()
				req := NewRequestWithBody(t, "PUT", groupURL, bytes.NewReader(pkgs[key])).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)

				req = NewRequest(t, "GET", groupURL+"/x86_64/test2-1.0.0-1-any.pkg.tar."+strings.ToLower(tp))
				resp := MakeRequest(t, req, http.StatusOK)
				require.Equal(t, pkgs[key], resp.Body.Bytes())

				req = NewRequestWithBody(t, "DELETE", groupURL+"/test2/1.0.0-1/any", nil).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusNoContent)
			})
		}
	}
	t.Run("Concurrent Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		var wg sync.WaitGroup

		targets := []string{"any", "aarch64", "x86_64"}
		for _, tag := range targets {
			wg.Add(1)
			go func(i string) {
				defer wg.Done()
				req := NewRequestWithBody(t, "PUT", rootURL, bytes.NewReader(pkgs[i])).
					AddBasicAuth(user.Name)
				MakeRequest(t, req, http.StatusCreated)
			}(tag)
		}
		wg.Wait()
		for _, target := range targets {
			req := NewRequestWithBody(t, "DELETE", rootURL+"/test/1.0.0-1/"+target, nil).
				AddBasicAuth(user.Name)
			MakeRequest(t, req, http.StatusNoContent)
		}
	})
}

func getProperty(data, key string) string {
	r := bufio.NewReader(strings.NewReader(data))
	for {
		line, _, err := r.ReadLine()
		if err != nil {
			return ""
		}
		if strings.Contains(string(line), "%"+key+"%") {
			readLine, _, _ := r.ReadLine()
			return string(readLine)
		}
	}
}

func listTarGzFiles(data []byte) (fstest.MapFS, error) {
	reader, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	tarRead := tar.NewReader(reader)
	files := make(fstest.MapFS)
	for {
		cur, err := tarRead.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if cur.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tarRead)
		if err != nil {
			return nil, err
		}
		files[cur.Name] = &fstest.MapFile{Data: data}
	}
	return files, nil
}

func gpgVerify(pub, sig, data []byte) error {
	sigPack, err := packet.Read(bytes.NewBuffer(sig))
	if err != nil {
		return err
	}
	signature, ok := sigPack.(*packet.Signature)
	if !ok {
		return errors.New("invalid sign key")
	}
	pubBlock, err := armor.Decode(bytes.NewReader(pub))
	if err != nil {
		return err
	}
	pack, err := packet.Read(pubBlock.Body)
	if err != nil {
		return err
	}
	publicKey, ok := pack.(*packet.PublicKey)
	if !ok {
		return errors.New("invalid public key")
	}
	hash := signature.Hash.New()
	_, err = hash.Write(data)
	if err != nil {
		return err
	}
	return publicKey.VerifySignature(hash, signature)
}
