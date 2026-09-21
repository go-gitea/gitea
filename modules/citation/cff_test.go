// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package citation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatCFF(t *testing.T) {
	apa, bibtex := FormatCFF(`cff-version: 1.2.0
authors:
  - family-names: Haines
    given-names: Robert
  - name: "The {curly_braces} Collective"
title: "Software that uses the following symbols: &, %, $, #"
version: 0.0.1-alpha
date-released: 2024-01-16
`)
	assert.Equal(t, "Haines, R., & The {curly_braces} Collective. (2024). Software that uses the following symbols: &, %, $, # (Version 0.0.1-alpha) [Computer software]", apa)
	assert.Equal(t, `@software{Haines_Software_that_uses_2024,
author = {Haines, Robert and {The \{curly\_braces\} Collective}},
month = jan,
title = {{Software that uses the following symbols: \&, \%, \$, \#}},
version = {0.0.1-alpha},
year = {2024}
}`, bibtex)

	apa, bibtex = FormatCFF(`cff-version: 1.2.0
authors:
  - family-names: Smith
    given-names: Arfon M.
title: "Software citation principles"
preferred-citation:
  authors:
    - family-names: Smith
      given-names: A. M.
    - name: "FORCE11 Software Citation Working Group"
  doi: "10.7717/peerj-cs.86"
  journal: "PeerJ Computer Science"
  month: 9
  start: e86
  title: "Software citation principles"
  type: article
  volume: 2
  issue: 123
  year: 2016
`)
	assert.Equal(t, "Smith, A. M., & FORCE11 Software Citation Working Group. (2016). Software citation principles. PeerJ Computer Science, 2(123), e86. https://doi.org/10.7717/peerj-cs.86", apa)
	assert.Equal(t, `@article{Smith_Software_citation_principles_2016,
author = {Smith, A. M. and {FORCE11 Software Citation Working Group}},
doi = {10.7717/peerj-cs.86},
journal = {PeerJ Computer Science},
month = sep,
number = {123},
pages = {e86},
title = {{Software citation principles}},
volume = {2},
year = {2016}
}`, bibtex)

	apa, bibtex = FormatCFF("cff-version: 1.2.0\ntitle: No authors\nauthor:\n  - name: Typo\n")
	assert.Empty(t, apa)
	assert.Empty(t, bibtex)
}
