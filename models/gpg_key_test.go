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
	assert.Nil(t, err, "Could not parse a valid GPG armored key", key)
	//TODO verify value of key
}
