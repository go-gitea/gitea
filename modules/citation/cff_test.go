// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package citation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatCFF(t *testing.T) {
	cases := []struct{ cff, apa, bibtex string }{
		{
			cff: `cff-version: 1.2.0
title: Overridden
authors:
  - family-names: Haines
    given-names: Robert
  - name: "The {curly_braces} Collective"
title: "Software that uses the following symbols: &, %, $, #"
version: 2024-01-16
license: [MIT, Apache-2.0]
date-released: 2024-01-16
`,
			apa: "Haines, R., & The {curly_braces} Collective. (2024). Software that uses the following symbols: &, %, $, # (Version 2024-01-16) [Computer software]",
			bibtex: `@software{Haines_Software_that_uses_2024,
author = {Haines, Robert and {The \{curly\_braces\} Collective}},
license = {["MIT", "Apache-2.0"]},
month = jan,
title = {{Software that uses the following symbols: \&, \%, \$, \#}},
version = {2024-01-16},
year = {2024}
}`,
		},
		{
			cff: `cff-version: 1.2.0
authors:
  - family-names: Smith
    given-names: Arfon M.
title: "Software citation principles"
license: MIT
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
  publisher:
    - name: The Open Journal
`,
			apa: "Smith, A. M., & FORCE11 Software Citation Working Group. (2016). Software citation principles. PeerJ Computer Science, 2(123), e86. https://doi.org/10.7717/peerj-cs.86",
			bibtex: `@article{Smith_Software_citation_principles_2016,
author = {Smith, A. M. and {FORCE11 Software Citation Working Group}},
doi = {10.7717/peerj-cs.86},
journal = {PeerJ Computer Science},
month = sep,
number = {123},
pages = {e86},
title = {{Software citation principles}},
volume = {2},
year = {2016}
}`,
		},
		{
			cff: `cff-version: 1.2.0
preferred-citation:
  type: conference-paper
  version: 1.10
  title: "Über tools"
  authors:
    - family-names: Ørsted
      given-names: hans christian
      name-particle: van
      name-suffix: Jr.
  collection-title: "Proceedings of X & Y"
  conference:
    name: "Conf_2020"
    city: Berlin
    country: DE
    date-start: 30.06.2020
    date-end: 2020-07-02
  start: 10
  end: 20
  editors:
    - family-names: Editor
      given-names: Eve
`,
			apa: "van Ørsted, H. C., Jr. (2020, June 30–July 2). Über tools (Version 1.1) [Conference paper]. Proceedings of X & Y, 10–20",
			bibtex: `@inproceedings{van_Orsted_Uber_tools_2020,
address = {Berlin, DE},
author = {van Ørsted, Jr., hans christian},
booktitle = {Proceedings of X \& Y},
editor = {Editor, Eve},
month = jun,
pages = {10--20},
series = {Conf\_2020},
title = {{Über tools}},
year = {2020}
}`,
		},
		{
			cff: `cff-version: 1.2.0
thesis: &thesis
  type: phdthesis
  title: Thesis
preferred-citation:
  <<: *thesis
  authors:
    - family-names: Nguyễn
      given-names: Sam
      affiliation: "Uni_A"
  status: in-press
  notes: A note
`,
			apa: "Nguyễn, S. (in press). Thesis. [Doctoral dissertation, Uni_A]",
			bibtex: `@phdthesis{Nguyn_Thesis_in_press,
author = {Nguyễn, Sam},
note = {A note},
school = {Uni\_A},
title = {{Thesis}},
year = {in press}
}`,
		},
		{cff: "cff-version: 1.2.0\ntitle: No authors\nauthor:\n  - name: Typo\n"},
	}
	for _, tc := range cases {
		apa, bibtex := FormatCFF(tc.cff)
		assert.Equal(t, tc.apa, apa)
		assert.Equal(t, tc.bibtex, bibtex)
	}
}
