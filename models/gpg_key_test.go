// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckArmoredGPGKeyString(t *testing.T) {
	testGPGArmor := `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFh91QoBCADciaDd7aqegYkn4ZIG7J0p1CRwpqMGjxFroJEMg6M1ZiuEVTRv
z49P4kcr1+98NvFmcNc+x5uJgvPCwr/N8ZW5nqBUs2yrklbFF4MeQomyZJJegP8m
/dsRT3BwIT8YMUtJuCj0iqD9vuKYfjrztcMgC1sYwcE9E9OlA0pWBvUdU2i0TIB1
vOq6slWGvHHa5l5gPfm09idlVxfH5+I+L1uIMx5ovbiVVU5x2f1AR1T18f0t2TVN
0agFTyuoYE1ATmvJHmMcsfgM1Gpd9hIlr9vlupT2kKTPoNzVzsJsOU6Ku/Lf/bac
mF+TfSbRCtmG7dkYZ4metLj7zG/WkW8IvJARABEBAAG0HUFudG9pbmUgR0lSQVJE
IDxzYXBrQHNhcGsuZnI+iQFUBBMBCAA+FiEEEIOwJg/1vpF1itJ4roJVuKDYKOQF
Alh91QoCGwMFCQPCZwAFCwkIBwIGFQgJCgsCBBYCAwECHgECF4AACgkQroJVuKDY
KORreggAlIkC2QjHP5tb7b0+LksB2JMXdY+UzZBcJxtNmvA7gNQaGvWRrhrbePpa
MKDP+3A4BPDBsWFbbB7N56vQ5tROpmWbNKuFOVER4S1bj0JZV0E+xkDLqt9QwQtQ
ojd7oIZJwDUwdud1PvCza2mjgBqqiFE+twbc3i9xjciCGspMniUul1eQYLxRJ0w+
sbvSOUnujnq5ByMSz9ij00O6aiPfNQS5oB5AALfpjYZDvWAAljLVrtmlQJWZ6dZo
T/YNwsW2dECPuti8+Nmu5FxPGDTXxdbnRaeJTQ3T6q1oUVAv7yTXBx5NXfXkMa5i
iEayQIH8Joq5Ev5ja/lRGQQhArMQ2bkBDQRYfdUKAQgAv7B3coLSrOQbuTZSlgWE
QeT+7DWbmqE1LAQA1pQPcUPXLBUVd60amZJxF9nzUYcY83ylDi0gUNJS+DJGOXpT
pzX2IOuOMGbtUSeKwg5s9O4SUO7f2yCc3RGaegER5zgESxelmOXG+b/hoNt7JbdU
JtxcnLr91Jw2PBO/Xf0ZKJ01CQG2Yzdrrj6jnrHyx94seHy0i6xH1o0OuvfVMLfN
/Vbb/ZHh6ym2wHNqRX62b0VAbchcJXX/MEehXGknKTkO6dDUd+mhRgWMf9ZGRFWx
ag4qALimkf1FXtAyD0vxFYeyoWUQzrOvUsm2BxIN/986R08fhkBQnp5nz07mrU02
cQARAQABiQE8BBgBCAAmFiEEEIOwJg/1vpF1itJ4roJVuKDYKOQFAlh91QoCGwwF
CQPCZwAACgkQroJVuKDYKOT32wf/UZqMdPn5OhyhffFzjQx7wolrf92WkF2JkxtH
6c3Htjlt/p5RhtKEeErSrNAxB4pqB7dznHaJXiOdWEZtRVXXjlNHjrokGTesqtKk
lHWtK62/MuyLdr+FdCl68F3ewuT2iu/MDv+D4HPqA47zma9xVgZ9ZNwJOpv3fCOo
RfY66UjGEnfgYifgtI5S84/mp2jaSc9UNvlZB6RSf8cfbJUL74kS2lq+xzSlf0yP
Av844q/BfRuVsJsK1NDNG09LC30B0l3LKBqlrRmRTUMHtgchdX2dY+p7GPOoSzlR
MkM/fdpyc2hY7Dl/+qFmN5MG5yGmMpQcX+RNNR222ibNC1D3wg==
=i9b7
-----END PGP PUBLIC KEY BLOCK-----`

	key, err := checkArmoredGPGKeyString(testGPGArmor)
	assert.NoError(t, err, "Could not parse a valid GPG public armored rsa key", key)
	//TODO verify value of key
}

func TestCheckArmoredbrainpoolP256r1GPGKeyString(t *testing.T) {
	testGPGArmor := `-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v2

mFMEV6HwkhMJKyQDAwIIAQEHAgMEUsvJO/j5dFMRRj67qeZC9fSKBsGZdOHRj2+6
8wssmbUuLTfT/ZjIbExETyY8hFnURRGpD2Ifyz0cKjXcbXfJtrQTRm9vYmFyIDxm
b29AYmFyLmRlPoh/BBMTCAAnBQJZOsDIAhsDBQkJZgGABQsJCAcCBhUICQoLAgQW
AgMBAh4BAheAAAoJEGuJTd/DBMzmNVQA/2beUrv1yU4gyvCiPDEm3pK42cSfaL5D
muCtPCUg9hlWAP4yq6M78NW8STfsXgn6oeziMYiHSTmV14nOamLuwwDWM7hXBFeh
8JISCSskAwMCCAEBBwIDBG3A+XfINAZp1CTse2mRNgeUE5DbUtEpO8ALXKA1UQsQ
DLKq27b7zTgawgXIGUGP6mWsJ5oH7MNAJ/uKTsYmX40DAQgHiGcEGBMIAA8FAleh
8JICGwwFCQlmAYAACgkQa4lN38MEzOZwKAD/QKyerAgcvzzLaqvtap3XvpYcw9tc
OyjLLnFQiVmq7kEA/0z0CQe3ZQiQIq5zrs7Nh1XRkFAo8GlU/SGC9XFFi722
=ZiSe
-----END PGP PUBLIC KEY BLOCK-----`

	key, err := checkArmoredGPGKeyString(testGPGArmor)
	assert.NoError(t, err, "Could not parse a valid GPG public armored brainpoolP256r1 key", key)
	//TODO verify value of key
}

func TestExtractSignature(t *testing.T) {
	testGPGArmor := `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBFh91QoBCADciaDd7aqegYkn4ZIG7J0p1CRwpqMGjxFroJEMg6M1ZiuEVTRv
z49P4kcr1+98NvFmcNc+x5uJgvPCwr/N8ZW5nqBUs2yrklbFF4MeQomyZJJegP8m
/dsRT3BwIT8YMUtJuCj0iqD9vuKYfjrztcMgC1sYwcE9E9OlA0pWBvUdU2i0TIB1
vOq6slWGvHHa5l5gPfm09idlVxfH5+I+L1uIMx5ovbiVVU5x2f1AR1T18f0t2TVN
0agFTyuoYE1ATmvJHmMcsfgM1Gpd9hIlr9vlupT2kKTPoNzVzsJsOU6Ku/Lf/bac
mF+TfSbRCtmG7dkYZ4metLj7zG/WkW8IvJARABEBAAG0HUFudG9pbmUgR0lSQVJE
IDxzYXBrQHNhcGsuZnI+iQFUBBMBCAA+FiEEEIOwJg/1vpF1itJ4roJVuKDYKOQF
Alh91QoCGwMFCQPCZwAFCwkIBwIGFQgJCgsCBBYCAwECHgECF4AACgkQroJVuKDY
KORreggAlIkC2QjHP5tb7b0+LksB2JMXdY+UzZBcJxtNmvA7gNQaGvWRrhrbePpa
MKDP+3A4BPDBsWFbbB7N56vQ5tROpmWbNKuFOVER4S1bj0JZV0E+xkDLqt9QwQtQ
ojd7oIZJwDUwdud1PvCza2mjgBqqiFE+twbc3i9xjciCGspMniUul1eQYLxRJ0w+
sbvSOUnujnq5ByMSz9ij00O6aiPfNQS5oB5AALfpjYZDvWAAljLVrtmlQJWZ6dZo
T/YNwsW2dECPuti8+Nmu5FxPGDTXxdbnRaeJTQ3T6q1oUVAv7yTXBx5NXfXkMa5i
iEayQIH8Joq5Ev5ja/lRGQQhArMQ2bkBDQRYfdUKAQgAv7B3coLSrOQbuTZSlgWE
QeT+7DWbmqE1LAQA1pQPcUPXLBUVd60amZJxF9nzUYcY83ylDi0gUNJS+DJGOXpT
pzX2IOuOMGbtUSeKwg5s9O4SUO7f2yCc3RGaegER5zgESxelmOXG+b/hoNt7JbdU
JtxcnLr91Jw2PBO/Xf0ZKJ01CQG2Yzdrrj6jnrHyx94seHy0i6xH1o0OuvfVMLfN
/Vbb/ZHh6ym2wHNqRX62b0VAbchcJXX/MEehXGknKTkO6dDUd+mhRgWMf9ZGRFWx
ag4qALimkf1FXtAyD0vxFYeyoWUQzrOvUsm2BxIN/986R08fhkBQnp5nz07mrU02
cQARAQABiQE8BBgBCAAmFiEEEIOwJg/1vpF1itJ4roJVuKDYKOQFAlh91QoCGwwF
CQPCZwAACgkQroJVuKDYKOT32wf/UZqMdPn5OhyhffFzjQx7wolrf92WkF2JkxtH
6c3Htjlt/p5RhtKEeErSrNAxB4pqB7dznHaJXiOdWEZtRVXXjlNHjrokGTesqtKk
lHWtK62/MuyLdr+FdCl68F3ewuT2iu/MDv+D4HPqA47zma9xVgZ9ZNwJOpv3fCOo
RfY66UjGEnfgYifgtI5S84/mp2jaSc9UNvlZB6RSf8cfbJUL74kS2lq+xzSlf0yP
Av844q/BfRuVsJsK1NDNG09LC30B0l3LKBqlrRmRTUMHtgchdX2dY+p7GPOoSzlR
MkM/fdpyc2hY7Dl/+qFmN5MG5yGmMpQcX+RNNR222ibNC1D3wg==
=i9b7
-----END PGP PUBLIC KEY BLOCK-----`
	ekey, err := checkArmoredGPGKeyString(testGPGArmor)
	assert.NoError(t, err, "Could not parse a valid GPG armored key", ekey)

	pubkey := ekey.PrimaryKey
	content, err := base64EncPubKey(pubkey)
	assert.NoError(t, err, "Could not base64 encode a valid PublicKey content", ekey)

	key := &GPGKey{
		KeyID:             pubkey.KeyIdString(),
		Content:           content,
		Created:           pubkey.CreationTime,
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}

	cannotsignkey := &GPGKey{
		KeyID:             pubkey.KeyIdString(),
		Content:           content,
		Created:           pubkey.CreationTime,
		CanSign:           false,
		CanEncryptComms:   false,
		CanEncryptStorage: false,
		CanCertify:        false,
	}

	testGoodSigArmor := `-----BEGIN PGP SIGNATURE-----

iQEzBAABCAAdFiEEEIOwJg/1vpF1itJ4roJVuKDYKOQFAljAiQIACgkQroJVuKDY
KORvCgf6A/Ehh0r7QbO2tFEghT+/Ab+bN7jRN3zP9ed6/q/ophYmkrU0NibtbJH9
AwFVdHxCmj78SdiRjaTKyevklXw34nvMftmvnOI4lBNUdw6KWl25/n/7wN0l2oZW
rW3UawYpZgodXiLTYarfEimkDQmT67ArScjRA6lLbkEYKO0VdwDu+Z6yBUH3GWtm
45RkXpnsF6AXUfuD7YxnfyyDE1A7g7zj4vVYUAfWukJjqow/LsCUgETETJOqj9q3
52/oQDs04fVkIEtCDulcY+K/fKlukBPJf9WceNDEqiENUzN/Z1y0E+tJ07cSy4bk
yIJb+d0OAaG8bxloO7nJq4Res1Qa8Q==
=puvG
-----END PGP SIGNATURE-----`
	testGoodPayload := `tree 56ae8d2799882b20381fc11659db06c16c68c61a
parent c7870c39e4e6b247235ca005797703ec4254613f
author Antoine GIRARD <sapk@sapk.fr> 1489012989 +0100
committer Antoine GIRARD <sapk@sapk.fr> 1489012989 +0100

Goog GPG
`

	testBadSigArmor := `-----BEGIN PGP SIGNATURE-----

iQEzBAABCAAdFiEE5yr4rn9ulbdMxJFiPYI/ySNrtNkFAljAiYkACgkQPYI/ySNr
tNmDdQf+NXhVRiOGt0GucpjJCGrOnK/qqVUmQyRUfrqzVUdb/1/Ws84V5/wE547I
6z3oxeBKFsJa1CtIlxYaUyVhYnDzQtphJzub+Aw3UG0E2ywiE+N7RCa1Ufl7pPxJ
U0SD6gvNaeTDQV/Wctu8v8DkCtEd3N8cMCDWhvy/FQEDztVtzm8hMe0Vdm0ozEH6
P0W93sDNkLC5/qpWDN44sFlYDstW5VhMrnF0r/ohfaK2kpYHhkPk7WtOoHSUwQSg
c4gfhjvXIQrWFnII1Kr5jFGlmgNSR02qpb31VGkMzSnBhWVf2OaHS/kI49QHJakq
AhVDEnoYLCgoDGg9c3p1Ll2452/c6Q==
=uoGV
-----END PGP SIGNATURE-----`
	testBadPayload := `tree 3074ff04951956a974e8b02d57733b0766f7cf6c
parent fd3577542f7ad1554c7c7c0eb86bb57a1324ad91
author Antoine GIRARD <sapk@sapk.fr> 1489013107 +0100
committer Antoine GIRARD <sapk@sapk.fr> 1489013107 +0100

Unknown GPG key with good email
`
	//Reading Sign
	goodSig, err := extractSignature(testGoodSigArmor)
	assert.NoError(t, err, "Could not parse a valid GPG armored signature", testGoodSigArmor)
	badSig, err := extractSignature(testBadSigArmor)
	assert.NoError(t, err, "Could not parse a valid GPG armored signature", testBadSigArmor)

	//Generating hash of commit
	goodHash, err := populateHash(goodSig.Hash, []byte(testGoodPayload))
	assert.NoError(t, err, "Could not generate a valid hash of payload", testGoodPayload)
	badHash, err := populateHash(badSig.Hash, []byte(testBadPayload))
	assert.NoError(t, err, "Could not generate a valid hash of payload", testBadPayload)

	//Verify
	err = verifySign(goodSig, goodHash, key)
	assert.NoError(t, err, "Could not validate a good signature")
	err = verifySign(badSig, badHash, key)
	assert.Error(t, err, "Validate a bad signature")
	err = verifySign(goodSig, goodHash, cannotsignkey)
	assert.Error(t, err, "Validate a bad signature with a kay that can not sign")
}

func TestCheckGPGUserEmail(t *testing.T) {
	testEmailWithUpperCaseLetters := `-----BEGIN PGP PUBLIC KEY BLOCK-----
Version: GnuPG v1

mQENBFlEBvMBCADe+EQcfv/aKbMFy7YB8e/DE+hY39sfjvdvSgeXtNhfmYvIOUjT
ORMCvce2Oxzb3HTI0rjYsJpzo9jEQ53dB3vdr0ne5Juby6N7QPjof3NR+ko50Ki2
0ilOjYuA0v6VHLIn70UBa9NEf+XDuE7P+Lbtl2L9B9OMXtcTAZoA3cJySgtNFNIG
AVefPi8LeOcekL39wxJEA8OzdCyO5oENEwAG1tzjy9DDNJf74/dBBh2NiXeSeMxZ
RYeYzqEa2UTDP1fkUl7d2/hV36cKZWZr+l4SQ5bM7HeLj2SsfabLfqKoVWgkfAzQ
VwtkbRpzMiDLMte2ZAyTJUc+77YbFoyAmOcjABEBAAG0HFVzZXIgT25lIDxVc2Vy
MUBFeGFtcGxlLmNvbT6JATgEEwECACIFAllEBvMCGwMGCwkIBwMCBhUIAgkKCwQW
AgMBAh4BAheAAAoJEFMOzOY274DFw5EIAKc4jiYaMb1HDKrSv0tphgNxPFEY83/J
9CZggO7BINxlb7z/lH1i0U2h2Ha9E3VJTJQF80zBCaIvtU2UNrgVmSKoc0BdE/2S
rS9MAl29sXxf1BfvXHu12Suvo8O/ZFP45Vm/3kkHuasHyOV1GwUWnynt1qo0zUEn
WMIcB8USlmMT1TnSb10YKBd/BpGF3crFDJLfAHRumZUk4knDDWUOWy5RCOG8cedc
VTAhfdoKRRO3PchOfz6Rls/hew12mRNayqxuLQl2+BX+BWu+25dR3qyiS+twLbk6
Rjpb0S+RQTkYIUoI0SEZpxcTZso11xF5KNpKZ9aAoiLJqkNF5h4oPSe5AQ0EWUQG
8wEIALiMMqh3NF3ON/z7hQfeU24bCl/WdfJwCR9CWU/jx4X4gZq2C2aGtytGN5g/
qoYQ3poTOPzh/4Dvs+r6CtHqi0CvPiEOfSxzmaK+F+vA0GMn2i3Sx5gq/VB0mr+j
RIYMCjf68Tifo2RAT0VDzn6t304l5+VPr4OgbobMRH+wDe7Hhd2pZXl7ty8DooBn
vqaqoKgdiccUXGBKe4Oihl/oZ4qrYH6K4ACP1Sco1rs4mNeKDAW8k/Y7zLjg6d59
g0YQ1YI+CX/bKB7/cpMHLupyMLqvCcqIpjBXRJNMdjuMHgKckjr89DwnqXqgXz7W
u0B39MZQn9nn6vq8BdkoDFgrTQ8AEQEAAYkBHwQYAQIACQUCWUQG8wIbDAAKCRBT
DszmNu+Axf4IB/0S9NTc6kpwW+ZPZQNTWR5oKDEaXVCRLccOlkt33txMvk/z2jNM
trEke99ss5L1bRyWB5fRA+XVsPmW9kIk8pmGFmxqp2nSxr9m9rlL5oTYH8u6dfSm
zwGhqkfITjPI7hyNN52PLANwoS0o4dLzIE65ewigx6cnRlrT2IENObxG/tlxaYg1
NHahJX0uFlVk0W0bLBrs3fTDw1lS/N8HpyQb+5ryQmiIb2a48aygCS/h2qeRlX1d
Q0KHb+QcycSgbDx0ZAvdIacuKvBBcbxrsmFUI4LR+oIup0G9gUc0roPvr014jYQL
7f8r/8fpcN8t+I/41QHCs6L/BEIdTHW3rTQ6
=zHo9
-----END PGP PUBLIC KEY BLOCK-----`

	key, err := AddGPGKey(1, testEmailWithUpperCaseLetters)
	assert.NoError(t, err)

	if assert.Len(t, key.Emails, 1) {
		assert.Equal(t, "user1@example.com", key.Emails[0].Email)
	}
}
