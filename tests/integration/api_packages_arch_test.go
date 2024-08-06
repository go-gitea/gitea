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
		"other": unPack(`
KLUv/QBYbRMABuOHS9BSNQdQ56F+xNFoV3CijY54JYt3VqV1iUU3xmj00y2pyBOCuokbhDYpvNsj
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
nYAR`),
	}

	t.Run("RepositoryKey", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", rootURL+"/repository.key")
		resp := MakeRequest(t, req, http.StatusOK)

		require.Equal(t, "application/pgp-keys", resp.Header().Get("Content-Type"))
		require.Contains(t, resp.Body.String(), "-----BEGIN PGP PUBLIC KEY BLOCK-----")
	})

	t.Run("Upload", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["any"]))
		MakeRequest(t, req, http.StatusUnauthorized)

		req = NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["any"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

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
		require.Len(t, pfs, 2) // zst and zst.sig
		require.True(t, pfs[0].IsLead)

		pb, err := packages.GetBlobByID(db.DefaultContext, pfs[0].BlobID)
		require.NoError(t, err)
		require.Equal(t, int64(len(pkgs["any"])), pb.Size)

		req = NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["any"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusConflict)
		req = NewRequestWithBody(t, "PUT", rootURL+"/default", bytes.NewReader(pkgs["x86_64"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithBody(t, "PUT", rootURL+"/other", bytes.NewReader(pkgs["any"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithBody(t, "PUT", rootURL+"/other", bytes.NewReader(pkgs["aarch64"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithBody(t, "PUT", rootURL+"/base", bytes.NewReader(pkgs["other"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithBody(t, "PUT", rootURL+"/base", bytes.NewReader(pkgs["x86_64"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
		req = NewRequestWithBody(t, "PUT", rootURL+"/base", bytes.NewReader(pkgs["aarch64"])).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusCreated)
	})

	t.Run("Download", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", rootURL+"/default/x86_64/test-1.0.0-1-x86_64.pkg.tar.zst")
		resp := MakeRequest(t, req, http.StatusOK)
		require.Equal(t, pkgs["x86_64"], resp.Body.Bytes())

		req = NewRequest(t, "GET", rootURL+"/default/x86_64/test-1.0.0-1-any.pkg.tar.zst")
		resp = MakeRequest(t, req, http.StatusOK)
		require.Equal(t, pkgs["any"], resp.Body.Bytes())

		req = NewRequest(t, "GET", rootURL+"/default/x86_64/test-1.0.0-1-aarch64.pkg.tar.zst")
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", rootURL+"/other/x86_64/test-1.0.0-1-x86_64.pkg.tar.zst")
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", rootURL+"/other/x86_64/test-1.0.0-1-any.pkg.tar.zst")
		resp = MakeRequest(t, req, http.StatusOK)
		require.Equal(t, pkgs["any"], resp.Body.Bytes())
	})

	t.Run("SignVerify", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", rootURL+"/repository.key")
		respPub := MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", rootURL+"/other/x86_64/test-1.0.0-1-any.pkg.tar.zst")
		respPkg := MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", rootURL+"/other/x86_64/test-1.0.0-1-any.pkg.tar.zst.sig")
		respSig := MakeRequest(t, req, http.StatusOK)

		if err := gpgVerify(respPub.Body.Bytes(), respSig.Body.Bytes(), respPkg.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Repository", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequest(t, "GET", rootURL+"/repository.key")
		respPub := MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", rootURL+"/base/x86_64/base.db")
		respPkg := MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "GET", rootURL+"/base/x86_64/base.db.sig")
		respSig := MakeRequest(t, req, http.StatusOK)

		if err := gpgVerify(respPub.Body.Bytes(), respSig.Body.Bytes(), respPkg.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
		files, err := listGzipFiles(respPkg.Body.Bytes())
		require.NoError(t, err)
		require.Len(t, files, 2)
		for s, d := range files {
			name := getProperty(string(d.Data), "NAME")
			ver := getProperty(string(d.Data), "VERSION")
			require.Equal(t, name+"-"+ver+"/desc", s)
			fn := getProperty(string(d.Data), "FILENAME")
			pgp := getProperty(string(d.Data), "PGPSIG")
			req = NewRequest(t, "GET", rootURL+"/base/x86_64/"+fn+".sig")
			respSig := MakeRequest(t, req, http.StatusOK)
			decodeString, err := base64.StdEncoding.DecodeString(pgp)
			require.NoError(t, err)
			require.Equal(t, respSig.Body.Bytes(), decodeString)
		}
	})
	t.Run("Delete", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()
		req := NewRequestWithBody(t, "DELETE", rootURL+"/base/notfound/1.0.0-1", nil).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequestWithBody(t, "DELETE", rootURL+"/base/test/1.0.0-1", nil).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequest(t, "GET", rootURL+"/base/x86_64/base.db")
		respPkg := MakeRequest(t, req, http.StatusOK)
		files, err := listGzipFiles(respPkg.Body.Bytes())
		require.NoError(t, err)
		require.Len(t, files, 1)

		req = NewRequestWithBody(t, "DELETE", rootURL+"/base/test2/1.0.0-1", nil).
			AddBasicAuth(user.Name)
		MakeRequest(t, req, http.StatusNoContent)
		req = NewRequest(t, "GET", rootURL+"/base/x86_64/base.db")
		MakeRequest(t, req, http.StatusNotFound)

		req = NewRequest(t, "GET", rootURL+"/default/x86_64/base.db")
		respPkg = MakeRequest(t, req, http.StatusOK)
		files, err = listGzipFiles(respPkg.Body.Bytes())
		require.NoError(t, err)
		require.Len(t, files, 1)
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

func listGzipFiles(data []byte) (fstest.MapFS, error) {
	reader, err := gzip.NewReader(bytes.NewBuffer(data))
	defer reader.Close()
	if err != nil {
		return nil, err
	}
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
