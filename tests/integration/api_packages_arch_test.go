// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"path"
	"testing"

	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestPackageArch(t *testing.T) {
	const firstPackage = `KLUv/QRYlUcA6nxwGD6gsLQNvOd6axb347rXM33zsZKpE9vgsf0Ld/n//99rY7sJi2vbpBtD3se3
qPGsw5p1E4nc33ohr0LLfwiK+m8BUgFTAdxyVWq/0afytiwo9jGT9PRVrkKxE/tIJX36mOzYG7Wv
cveuyTpPM0VnlTUUv80UWkllNk8Tq7PKL8Sb6bbk0hjizOq2xCR97KvMnb1Ul3dztp2HuEjs2egb
uesZ3W+P2oIaN23sMXUNXg7nDnfbeRbFzuFsxDvR03h3fqkuHoq4a7Eo/DA3anWX1V1WZ7vOaItZ
7Lquc+wZXaorKbz152+3ZcFLdWVR7H3bzou+lj7al49HFxRZR5fq2piiNtQpNVMbvVTX7dYr2jGu
UEkMWyGNS3XtIovSVprGddxJsxidXmo1VxlGM5vR1Mmzp0yrp9XqrjQ7T/duKXuyM5t281IrLts8
eqkucOSbVi2L5/AyHrMtx4ZaulQXq9v2a+kiKumluvZN4pb7WrrofrbVWX2prgva8Gvponqprm7B
La6uLop7TgDbdj7OC0B2wiqdFz1pWZbVLcvopWlbpRPAlWXbnBMAUgoYLE64YEGjxAfbByo5v3nW
p1LjdguTngi2fnvKdFse19roT9nJUOQZ/SpDkfPQ7tgqpRqjnn6gkt/M2a5x9pust4/i1vixkxy7
VN1m16+0ocX1hP2UtltPiVXpU6lxEyZvxuO3WombPib13R+tT+UFP/3x3Mfwy75+lZ8eQw1+J+yr
7Dict9NXWTmc60/Zl27v98PqUwlyHudwll33VSq7rPu+viZbSvbLlm6en/rMC7Gn88HbORO/Ga+y
7DV5M15V7Knsorhvy6661Ut/yp5K/olb81W9kz62WmGvPja/yn21tqC8HU663nxNduzdb+Qu6jP6
VPIRVR97TWKvcV7Fvv4mOU/Or4+SeY/d9lXqsV/eNdPGPnW3mHpM2pxxkwhqMzVrnVXy222HV7jv
20nboarGTZXK1M+nEucu7vqox2Qo8k3O7KvkGfcxxFwmNzqfviZZVOabdD1M3h3HINfzM/mFkutO
3FrUqKdfSzPtHt4Q6w17TeZ9dfaTynp3p3E3qdz+9FWyum71pLXOClqNvfZUglajOsd5i3tj8CRl
aMHO6sx5eLX4VLK6DlO937BqObLfJL98j11I9c+vn8m90mD2WYp+9lRmYCpryr9Jvq9nL0tibSvl
4e0o/SprBpc27LPPlh77KcW+bxKH875dg7ePHM6d43C0K1R36ech83D67KvMotg37in6maQhtx2V
PSbvHru2nVyJW3MmzuFcWZzPu7vLNSh23eWomrnlVE/baQ5n3JKUaixrWu+sL6y9hjj3G67CVmd3
D0Oxy7Fvz3jVb8jhTKnsNKj9fCpzCHI4v5ctx1v1p8wcczhnNfTwFve+8tPfpE/lx+GcPSbxasXh
nH2VPbxJF0V/ylW+HM6on/ILbcfhrDkO5y20I+eWw/n0mLxJmNVty+GMBUXzUl23CXNR3320b6gk
HoFi9zdr0C7do6MPXPfOulRX3mMR3h2/3pXH1eU0LtXV8etdoYj5vknXF2I+XheYuMu73e4bXhXX
hVNGaz1hKlrE+AB5hBg/RgQQAsPFQRQFf26EsFG04gwPaqAAlSeClrQACD4s2bkwg9uzJgTSDBxE
FtQ41OHTIlDYgLMqcdA8UEMDwRsrbA7NJXgOzZkjYgKI+WWRoiYICyk45FRxM8NEkTxZBBBMAkDB
oKYNBuFCgA4zQxYlE5DOHLiyxaQw02JKmRNDJuCw00NGi+4ETQMCSLQM+gAmYOI1aACo0BUygUeY
OgxWZEmyBQYWMYEQIbLixRDSEi584GARQAZNjid9hiCgWnzQEdTkI4kVMnoUgJFwwNwp9CfCKQ2g
HU2iQOHwAwkpgAoeAnhs0CDoi44KEgQQiGFUoIuX1ptEZgCxGBPCEBAEJAbA+JLExwADRyzQeqLY
zOZEgULGjBfvdd5Ev3I4N2WvyaYlWjFLCfsqM+5jh9VZmzrlvMsK6n6S4irES7djT+W2HRS7JM7E
T49pVXr6M/qpp1L0Rl47zjw4/UwqiZ5l8c+kd/mcN8TZs3c/VmWBFaiRHKSYQggxNDMjBUmyHHEE
hCDonKSQDxJAUFGMQRAIQxAEQhCGolCgZsokSJCknBQWWgMMyBbo9PMBsFwQnPDPtvRb/dr0T6ca
rtWEfRFesPpr1z8Hfi1LxlZ7GHuAqLAB/NNkT2fx9grmc8euvTXOAFacJCHIHICRPX0dBGk3RXcV
O9Vbi1MpQyibg34fKNSG4gIAImaNUbIu0zfT9FobmWjMOOItiiByrLEAB6d8YsQZbHhLrsELnaDN
YwLqgJZWL5Vift1uHpg7gRyXkl2anG/zeyw1mIGTPwyZqFfredyRUiNx6NtKPFgwNYHNOPVomgJM
BaZp2FQUdJDBjT42M7a1kJEekgrdLcA1m0WTzMdEe+7KcX/e8MttkzdAsRKuH4mndiUf8jnMxYj8
J80ggDEVFkFGIIOGiFOY0GDf8XoZ0rarlrzAoXFcSQfwQzQE6hBdPKxtrb3F5CUsCvwdwovq+VMV
5/sQEYCRaRsDohAoCw3tOMLg9pbOS+5ARZR9JDx5ScgyXOo5O6pbsBi1V7aja81BSEGZQFzHzpIv
NvXb/U5ZNldpCgWmyJfNGgiJJwIzzEgG5DChlpsh2W07AybfEiJWG8YZTMRChZk1dCnsTGxXdAKb
yEkYN9mks01OZ1gyVQaSakNg88h7upWYM40YIx4EJuyAwSP1yhfZPk6ZxNU3TrQygytGzwUxS2QO
IkJ4YHo8TXvkN5vq2aWjzeEJBFWQHV5rkrC1HkgK9Zk0B3RoSs2/mBmuI0iGSroNA/xAictf1DqT
0HVqqjQg1ACXRABLTDRjhUSa9kjdMcX9YypiMXY4iECBSl4oQeIQ112zLCppoz+HULQL+csQyVJc
CLajh0i2JJvzlFnmg2XIndtEIyTU7MyAiE8KO7SOxFc9mlYAyXKFFBFJkas615qu3PEr6bRA27AE
weALS7CuDOK6KF6YdYvOWdiCAoHI6xI=`
	firstPackageData, err := base64.StdEncoding.DecodeString(firstPackage)
	assert.NoError(t, err)

	const firstPackageSignature = `iQEzBAABCAAdFiEEdez1wBMjH7w+s6HCOjpq1/IBK5QFAmTKScUACgkQOjpq1/IBK5RDjwf/YZQS
QM9JgxtNqp9jxT7eqyNtYY5Jwte6Fpq6RpOd2qbkJodJVrAp0HAWPS71W9k0lhvOSeq4hL7jufUR
y5gmbvmN6CqOjoMAnSxe51OKgZuPb8fbWrpt58BqtR7iCtPav1tMG9lpIPWSLS2/jxoTIxjcgVQJ
05s2bqUtpoDy5fCB2Y5tdIPQbMjSr6/TkmWg4ulwactJg46bWgowwKxnWzUx7IYPjC3lwknU6Mll
DArW/X0zMaFT3zuMBJFlbSzv59tcH0yICa1yMRtWCnbufZo6Q/BUvZ3P3Wr/APokAEt5U1U/u0EK
Qck04tbpECPL0eABIygaLmwqii6wX6NIYA==`
	firstPackageSignatureData, err := base64.StdEncoding.DecodeString(firstPackageSignature)
	assert.NoError(t, err)

	const firstPackageDatabase = `H4sIAAAAAAAA/+ySQW+cMBCF9zy/Yi9zBMZgbOiNAm1Qd9MISg+9VLbXTqNNAAFRov76it2qavZa
RVUkvst4rJH9/PwGZY7q1nrMY8HBTmbzChARSSlPlYguK8WC/Vmf9hmTXGy29BpiLnmcZjVu6J/v
unzcGwE/VLvyOtuXCH9FwXtOxHfB/eF4689q9H9OMwC+GAPA91nzov9a1k31+RqBeQwAi7LJEdrJ
usf77ZKt8W6Y7/oOAPOm+lYihBFFAFidOwLAfRE37R5BHwxLnYoY06lUWnCKJFfGakYsTEQIgM1V
FsbiNB0ro0MTkTah0TzRsSXhbHhwziklVWStIKYkiSTlSlspuBBSac0Sbo1zi9i23iH8mOdhehcE
9lk9DPfWN/1D0D91dgxGO/QAmNX5FcLZnMWAttoVRfalRGAipVQmIlkOu8nyT9nHskZou2PXP3Xb
m7NJI8D//vCVlZWV3/wKAAD//xmJdqQACAAA`
	firstPackageDatabaseData, err := base64.StdEncoding.DecodeString(firstPackageDatabase)
	assert.NoError(t, err)

	const secondPackage = `KLUv/QRYPUcA+nxYGD6gsLQN5PnPkjy6fXl2KEtLb63ASYEmmw08uPr///fa2G7C4lq3UGMDqi4V
ALcz1mwbtcj9A2UA4JPvxl6MF28BUgFTAVuVp9foU3lbFhT7mEmK+iq9UOTEPlJJnz4mO+7G01e5
ez/JOlFTRWeVNRQ/ptBKKrOJmlidVX4h3ky3JZfGEGcWtzMNh5Zakpikj32VmbPX6upuzrbzEBuJ
PUf4xu12Ee63R21BjZs27pi6Bu+GM4e77TyLYt9wjsA5sdN4d36tLh6KuGvRKPwwj9DiLou7LM5y
XIQtZpHjuM5xF+FaXUnhrT9fuy0LXqsri2Lv23Zu9LX00b58PLqgyDq6VtfGKhQqhVJNlUav1XW7
7Yz2iitUEsNWCONaXdtIqx52utxmOxQKq/NyqDlPk1NdT5upelNdVzN7563b9LTZ0aqqt6ObpZvK
qyltXqsLHLl2qpbFcXQZj0yhvVYXi9v2a2kjKum1uvZN4nb7WtrofrbFWX2trgva8Gtpo3qtrm7B
LXpXF8U9J4BtOx/nBYCisErnhWGoLMuqlmX0qiet0gngyjJtzglAxYgUJUIglYDx4QNUcn7zrE/l
adstTHYi2HrtKdNtdVtLoz8lJ0ORZ/SrDEXOQ6tDq5RqnHq6gUp+82a7xtlrst4+ilvjx1By5FRV
m1y/0oYWVxT2U9puOyVWpU+lxk2YvBmPn+eJmz4m9d0frU/lBT/98dzH8Mu+fpWfHkMNfijsq+Q2
nDXUV1k3nOtP2Zdu7/fD6lMJbh3fcJYc91UquYz7vv5JtpTsly3dPF/1WRdiT+eDt3MmfjP2suxP
8mbsVeyp7KK4b8t63eqlR2VPJf/ErblX76SPeR7u6mPzq9xXawvKy+Gk280/yY67+43bTX1Gn0o+
pupjf5LYn7auYl9fk1sn59dPybxHTvsq9dgv75ppY6+6W1Q9Jm3OuEkET1M1a51V8tsthz3c9+Wk
5VJV46ZKpernU4lzF3f91GMyFLkmZ/ZV8oz7GOItkxqdT/8kWVTmm3Q7TF5uAxjken4mv1ByzYlb
izr19Gtppt3DG2KtYX+SeV+d/aSy3s1pzE0qtUd9lSyuWz1prbOCVmN/eipBq1Od47zFvTGIkjK0
IGd15jy8WnwqWV2Hqt5vWE84stckv3yPXEj1z6+fye1pMPtMRT97KjNQlTXl1yTft7OXJbG2lfLw
cpR+lTWDSxr22WdLj/2UYt83acN5367B28cN585xOFovxV36ZcgyoD77KrMo9o27in4machtT2WP
ybvHri0nPXFrzsQ3nCuL83k3d7kGRY67PFUzt5zqaTm94YxbklKN5dOpd9YX1l5DnPsNvbDF2d3D
UORw7Nsz9voNN5wplZwGTz+fyhyCG87fZbvxVv0pM8cbzlkNO7zFva/89DfpU/ltOGePSex5G87Z
V9nDm3RT9Kf08t1wTv2UX2i5DWe9bThroR05txvOqMfkTcIsbtsNZ0zbEEbzWl23CXNR3320b6gk
HoEi9zdr0C7do6MNXPfOulZX3qMR3hy/3ZVH73IY1+ri+O2uUMR836TrCzEfLwlM29XdbvcNL4lL
AnaiFaVhJE6s+AA5TI4VOACRlwlwBHRJZLDZFE5sMDLFEJgsTOio4BW2DUh4ggBFXPxMudLSYsTk
A0icCxiUSJAyxsqXITdYFLikk4ElMkhm+CjCgQOPgkS2KYD8kxTgBYiKBMlLnDcubCBJ05oRINWA
BtLop45XosUKC2AUOFlY6AmiQYQDzwmLPE5ScPCg0aEFyBkuIR7cQX0OliUtI0g4QEDKEkRMCB05
7gbBe2KVlHwh8qKDDlHm+AgJNQmyJkoUpTc/BHR1zKjpU4jDEaArJ0H2jqEYcpyoeiQIEqgIEniA
gVIVN13ssNA19pTYEkBQlwWqECPmUElz6MgvKrNBWRSIAAIPA0yMacNDQAEhECqKYjObU4SIFi9Q
fNd5E/264dyU/Uk2LdGKWUrYV5lxHzmsztrUKeddVlB3lBS9EC/djj2V23ZQ5JI4E0c9dqqy05/R
Vz2VYjfy2nHewelnUknsLIt/JrvL57whzp29+7EqOR6BFKiRHKSYQggxNDMjBUmyHHEEhCDonKSQ
DxJAcFGKQRAIQxAGQhCGolCgZsoUFBSknNJCaxLIFuLu8wEs1xIq/L/t/VY/OP3dqYbLsrBfxw5W
cvD6s/Cv/cvYagebBwgUG8B/WPN0YPGWImZwya7tsnkAK6GnndwczJA9fR0EafdFdxk7lVqLXylD
l81lvycUSkMRAwBBs8Y2WZfpW2l6rR1MNGYc8RZFEDnFWLCDUwoxkgw2uiU3wUtO0PoxwXRAS6uf
SjG6bucH5mYgx0nJPk1O2/zuyxjMwMlfhkzEqzU17siikQD0bSkeNJiaYjP6/Z6mwFcBaBqWFQWF
ZXDvjs28bS1upEdSobsFeM1mwSQzmWjKXXncnxk+c9ukDVCEhOtU4ilNyQd2DqMwk/+sGQR0TAVE
kDHIoCPi1CQ0CHe8V4Y029VBXuBsHLfcAfwQDUUdlotXa9trfzGZCYsOf0d4UT9/CsX5HiKCsD1t
k0UUDmVhooUjEDzfknjJEKiIvo+EJ4cJeYZLNmfXdAsWo+XKdvSsOVApMBMI4dgh+YKmfufflOV+
lV6hgBS5s1mDkPhE4MKMeHAOk2y5GZLdujOA+ZYnYsUwXsEELFSYWUuX8s5cdUVX2ASdhGmT7Tpz
cnrHkqk1wFTbAptj3tOthM9UxRjhINCwAwweiSv/yHY5ZR5XxzhRywxiGb0Q3CwROUgI4Yfp8TTt
kV/aVGSXqjaHJxCpIH14LUhiaz0jKdBnbA7UoSmYf+FmGI7gG6rlNgzwAyUuf1HrTCbXKVOlAKEe
XF4BLD3RTCsc0nRH2o5p6R9rEdvYiyBiDyqpogShQxzsmnGpBOiHtFETHT+pV6+EY8/bCavohoSI
88iZNYTdzZ3RRGogZlPW6YbdqMPzqp1J64BkWgkRjSRHVMMs/JVreyWdFvgalsYYrLAH1s+Ir4vi
hVm78J3RLCgQ9mBp`
	secondPackageData, err := base64.StdEncoding.DecodeString(secondPackage)
	assert.NoError(t, err)

	const secondPackageSignature = `iQEzBAABCAAdFiEEdez1wBMjH7w+s6HCOjpq1/IBK5QFAmTKSigACgkQOjpq1/IBK5TnkQgAotU2
kdi275k87qzRLp4SgOX4QTpwCyjmXK4ZEm/FGBiF84mYT/sQmKbSsxPbxd4lumHhbll5SMVdM3C+
1pB4kWT1fewQi1YukGEe+Na6SAa33yQqQThf30WPYJhuOxSNDX0DNCrR7Ei98hNq0FuvZkZWzepF
ylhUP+OSGPWFsVOnXCaANxW6457LtnNPeQFDwQL2y9qv0Hgpnn3KS09n0SPXT2Kr02iYu9rICoOB
KsqYqMGxHoPypwT24o6oDElGt9/Z0pqZcwRJc7C0npv3q5a17S7nAdo8/NWgfDri7w224s8odtWy
qTAeEmTTQ8awXFYounZaHg+455+8u5npkw==`
	secondPackageSignatureData, err := base64.StdEncoding.DecodeString(secondPackageSignature)
	assert.NoError(t, err)

	const secondPackageDatabase = `H4sIAAAAAAAA/+zSzW7bMAwA4Jz5FLnwmJhyLMnazUu81VjSFfG8w276oboirW3YLrru6YemwLDk
OhTDgHwXigIBiRJ76w/2lhdiIZLAo5+9ASIirfUxEtF5JClXv9fHfSF0pmdzeovLnHscJzvM6K/P
Om/uP4Efqm15XexKhD9GYWHb52V/uF1Odlj+HCcAPKkBwPdFfZJ/Lfd19fkaQSwEAG7Keo3QjBwf
7+cvgzXc9dNd1wLguq6+lQhpalIArF4zAsDdRtbNDoFXPmfjNbsQg/UyzRWFXLAwdsUx8wBYXxWp
VMfqVfQZp9GYYJ0yLhB7zYG0IymEiuykCqlWlHpnQ5Y5qzgXLroQZRZUFgGw2W8Rvk9TP75LEv5h
H/p7XvruIemeWh6SgfsOAIv9+grBts8v3TfVdrMpvpQIQhkyOtfaAOBNsf5UfCz3CE17aLundn7z
+kIDwL/+6ouLi4sTvwIAAP//Stn2gwAIAAA=`
	secondPackageDatabaseData, err := base64.StdEncoding.DecodeString(secondPackageDatabase)
	assert.NoError(t, err)

	assert.NoError(t, unittest.PrepareTestDatabase())

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})

	rootURL := fmt.Sprintf("/api/packages/%s/arch", user.Name)

	t.Run("PushFirstPackage", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "PUT", path.Join(rootURL, "/push", "package-1-1-x86_64.pkg.tar.zst", "archlinux", hex.EncodeToString(firstPackageSignatureData)))

		req.Header.Set("Content-Type", "application/octet-stream")
		req.Body = io.NopCloser(bytes.NewReader(firstPackageData))
		req = AddBasicAuthHeader(req, user.Name)

		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("GetFirstPackage", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", path.Join(rootURL, "/archlinux/x86_64/package-1-1-x86_64.pkg.tar.zst"))

		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, firstPackageData, resp.Body.Bytes())
	})

	t.Run("GetFirstDatabase", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", path.Join(rootURL, "/archlinux/x86_64/"+fmt.Sprintf("%s.%s.db", user.Name, setting.Domain)))

		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, firstPackageDatabaseData, resp.Body.Bytes())
	})

	t.Run("GetFirstSignature", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", path.Join(rootURL, "/archlinux/x86_64/package-1-1-x86_64.pkg.tar.zst.sig"))

		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, firstPackageSignatureData, resp.Body.Bytes())
	})

	t.Run("PushSecond", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "PUT", path.Join(rootURL, "/push", "package-1-1-any.pkg.tar.zst", "archlinux", hex.EncodeToString(secondPackageSignatureData)))

		req.Header.Set("Content-Type", "application/octet-stream")
		req.Body = io.NopCloser(bytes.NewReader(secondPackageData))
		req = AddBasicAuthHeader(req, user.Name)

		MakeRequest(t, req, http.StatusOK)
	})

	t.Run("GetSecondPackage", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", path.Join(rootURL, "/archlinux/any/package-1-1-any.pkg.tar.zst"))

		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, secondPackageData, resp.Body.Bytes())
	})

	t.Run("GetSecondDatabase", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", path.Join(rootURL, "/archlinux/any/"+fmt.Sprintf("%s.%s.db.tar.gz", user.Name, setting.Domain)))

		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, secondPackageDatabaseData, resp.Body.Bytes())
	})

	t.Run("GetSecondSignature", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "GET", path.Join(rootURL, "/archlinux/any/package-1-1-any.pkg.tar.zst.sig"))

		resp := MakeRequest(t, req, http.StatusOK)

		assert.Equal(t, secondPackageSignatureData, resp.Body.Bytes())
	})

	t.Run("Remove", func(t *testing.T) {
		defer tests.PrintCurrentTest(t)()

		req := NewRequest(t, "DELETE", path.Join(rootURL, "/remove", "package", "1-1"))

		req = AddBasicAuthHeader(req, user.Name)

		MakeRequest(t, req, http.StatusOK)
	})
}
